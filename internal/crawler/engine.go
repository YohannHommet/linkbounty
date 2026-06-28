package crawler

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocolly/colly/v2"
)

const userAgent = "LinkBountyBot/1.0 (+https://linkbounty.io/bot)"

// Config holds the tunable limits for a crawl. Use DefaultConfig for production
// values; tests construct small configs to exercise cap behaviour cheaply.
type Config struct {
	MaxPages         int           // max internal pages to crawl
	MaxExternalLinks int           // max external links to verify
	ExternalWorkers  int           // parallel external-link checkers
	MaxDepth         int           // colly crawl depth
	RequestTimeout   time.Duration // per-request timeout for external checks
}

// DefaultConfig returns the production crawl limits.
func DefaultConfig() Config {
	return Config{
		MaxPages:         100,
		MaxExternalLinks: 500,
		ExternalWorkers:  20,
		MaxDepth:         4,
		RequestTimeout:   8 * time.Second,
	}
}

// Link describes a checked external link and its outcome.
type Link struct {
	SourcePage string
	TargetLink string
	StatusCode int
	ErrorMsg   string
	LinkType   string // "broken" | "redirect" | "unverifiable"
}

type ProgressFunc func(pages int)

// Result holds everything the caller needs after a crawl.
type Result struct {
	Links         []Link
	PagesCrawled  int
	ExtLinksFound int // total external links encountered, including those skipped by cap
}

// dialContextFunc is the dial function used for external link checks.
// Tests override this with a permissive dialer to allow httptest.Server on 127.0.0.1.
var dialContextFunc = safeDialContext

// SetDialContextFunc replaces the dial function used by external link checks.
// Intended for tests only.
func SetDialContextFunc(fn func(ctx context.Context, network, address string) (net.Conn, error)) {
	dialContextFunc = fn
}

// safeDialContext wraps the default dialer and rejects resolved private/loopback
// addresses on every dial, preventing DNS-rebinding SSRF.
func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, rawIP := range ips {
		ip := net.ParseIP(rawIP)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return nil, fmt.Errorf("SSRF: connection to private address %s denied", ip)
		}
	}
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return d.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
}

func safeTransport(workers int) *http.Transport {
	return &http.Transport{
		DialContext:         dialContextFunc,
		MaxIdleConnsPerHost: workers,
		MaxIdleConns:        200,
		IdleConnTimeout:     30 * time.Second,
	}
}

// Run crawls startURL with the default production config.
func Run(ctx context.Context, startURL string, onProgress ProgressFunc) (Result, error) {
	return RunWithConfig(ctx, DefaultConfig(), startURL, onProgress)
}

// RunWithConfig crawls startURL with explicit limits.
func RunWithConfig(ctx context.Context, cfg Config, startURL string, onProgress ProgressFunc) (Result, error) {
	parsed, err := url.Parse(startURL)
	if err != nil {
		return Result{}, fmt.Errorf("invalid url: %w", err)
	}
	domain := parsed.Hostname() // for AllowedDomains (no port)
	siteHost := parsed.Host     // host:port — distinguishes same-host servers on different ports

	var (
		links     []Link
		mu        sync.Mutex
		pageCount atomic.Int64
		extSem    = make(chan struct{}, cfg.ExternalWorkers)
		extCount  atomic.Int64
		extWg     sync.WaitGroup
	)

	client := &http.Client{
		Transport: safeTransport(cfg.ExternalWorkers),
		Timeout:   cfg.RequestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // capture redirects, don't follow
		},
	}

	c := colly.NewCollector(
		colly.AllowedDomains(domain),
		colly.MaxDepth(cfg.MaxDepth),
		colly.Async(true),
		colly.UserAgent(userAgent),
	)
	c.IgnoreRobotsTxt = false // respect robots.txt

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		RandomDelay: 200 * time.Millisecond,
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		raw := e.Attr("href")
		abs := e.Request.AbsoluteURL(raw)
		if abs == "" || strings.HasPrefix(raw, "#") {
			return
		}

		target, err := url.Parse(abs)
		if err != nil {
			return
		}

		// skip non-HTTP schemes — mailto:, tel:, javascript:, ftp:, etc.
		if target.Scheme != "http" && target.Scheme != "https" {
			return
		}

		if strings.EqualFold(target.Host, siteHost) {
			// internal page — visit if under limit
			if pageCount.Load() < int64(cfg.MaxPages) {
				n := pageCount.Add(1)
				if onProgress != nil {
					onProgress(int(n))
				}
				e.Request.Visit(abs)
			}
			return
		}

		// external link — always count, only check if under cap
		n := extCount.Add(1)
		if n > int64(cfg.MaxExternalLinks) {
			return
		}

		extWg.Add(1)
		go func(sourcePage, targetLink string) {
			defer extWg.Done()
			select {
			case extSem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-extSem }()

			result := checkExternal(ctx, client, sourcePage, targetLink)
			if result != nil {
				mu.Lock()
				links = append(links, *result)
				mu.Unlock()
			}
		}(e.Request.URL.String(), abs)
	})

	if err := c.Visit(startURL); err != nil {
		return Result{}, fmt.Errorf("visit failed: %w", err)
	}
	c.Wait()
	extWg.Wait()

	return Result{
		Links:         links,
		PagesCrawled:  int(pageCount.Load()),
		ExtLinksFound: int(extCount.Load()),
	}, nil
}

func checkExternal(ctx context.Context, client *http.Client, sourcePage, targetLink string) *Link {
	code, err := headWithGETFallback(ctx, client, targetLink)
	if err != nil {
		return &Link{
			SourcePage: sourcePage,
			TargetLink: targetLink,
			StatusCode: 0,
			ErrorMsg:   err.Error(),
			LinkType:   "broken",
		}
	}

	switch {
	case code >= 200 && code < 300:
		return nil // OK
	case code >= 300 && code < 400:
		return &Link{
			SourcePage: sourcePage,
			TargetLink: targetLink,
			StatusCode: code,
			LinkType:   "redirect",
		}
	case code == 999:
		// 999 is LinkedIn's (and some CDNs') anti-bot response — the link is likely valid
		return &Link{
			SourcePage: sourcePage,
			TargetLink: targetLink,
			StatusCode: code,
			ErrorMsg:   "bot-blocking response (999) — link likely valid",
			LinkType:   "unverifiable",
		}
	default:
		return &Link{
			SourcePage: sourcePage,
			TargetLink: targetLink,
			StatusCode: code,
			ErrorMsg:   fmt.Sprintf("HTTP %d", code),
			LinkType:   "broken",
		}
	}
}

// headWithGETFallback tries HEAD first; falls back to GET on network error or 405/501.
func headWithGETFallback(ctx context.Context, client *http.Client, targetLink string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetLink, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		// HEAD failed — fall back to GET
		return doGET(ctx, client, targetLink)
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	if code == http.StatusMethodNotAllowed || code == 501 {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
		return doGET(ctx, client, targetLink)
	}
	return code, nil
}

func doGET(ctx context.Context, client *http.Client, targetLink string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetLink, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
