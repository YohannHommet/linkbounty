package crawler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocolly/colly/v2"
	"linkbounty/internal/database"
)

const (
	maxPages         = 100
	maxExternalLinks = 500
	externalWorkers  = 20
	userAgent        = "LinkBountyBot/1.0 (+https://linkbounty.io/bot)"
	requestTimeout   = 8 * time.Second
)

type ProgressFunc func(pages int)

// Result holds everything the caller needs after a crawl.
type Result struct {
	Links         []database.BrokenLink
	PagesCrawled  int
	ExtLinksFound int // total external links encountered, including those skipped by cap
}

func Run(startURL string, onProgress ProgressFunc) (Result, error) {
	parsed, err := url.Parse(startURL)
	if err != nil {
		return Result{}, fmt.Errorf("invalid url: %w", err)
	}
	domain := parsed.Hostname() // for AllowedDomains (no port)
	siteHost := parsed.Host     // host:port — distinguishes same-host servers on different ports

	var (
		links     []database.BrokenLink
		mu        sync.Mutex
		pageCount atomic.Int64
		extSem    = make(chan struct{}, externalWorkers)
		extCount  atomic.Int64
		extWg     sync.WaitGroup
	)

	client := &http.Client{
		Timeout: requestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // capture redirects, don't follow
		},
	}

	c := colly.NewCollector(
		colly.AllowedDomains(domain),
		colly.MaxDepth(4),
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
		if abs == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "javascript:") {
			return
		}

		target, err := url.Parse(abs)
		if err != nil {
			return
		}

		if strings.EqualFold(target.Host, siteHost) {
			// internal page — visit if under limit
			if pageCount.Load() < maxPages {
				pageCount.Add(1)
				if onProgress != nil {
					onProgress(int(pageCount.Load()))
				}
				e.Request.Visit(abs)
			}
			return
		}

		// external link — always count, only check if under cap
		n := extCount.Add(1)
		if n > maxExternalLinks {
			return
		}

		extWg.Add(1)
		go func(sourcePage, targetLink string) {
			defer extWg.Done()
			extSem <- struct{}{}
			defer func() { <-extSem }()

			result := checkExternal(client, sourcePage, targetLink)
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
		Links:        links,
		PagesCrawled: int(pageCount.Load()),
		ExtLinksFound: int(extCount.Load()),
	}, nil
}

func checkExternal(client *http.Client, sourcePage, targetLink string) *database.BrokenLink {
	req, err := http.NewRequest(http.MethodHead, targetLink, nil)
	if err != nil {
		return &database.BrokenLink{
			SourcePage: sourcePage,
			TargetLink: targetLink,
			StatusCode: 0,
			ErrorMsg:   err.Error(),
			LinkType:   "broken",
		}
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		// HEAD failed — try GET
		req2, _ := http.NewRequest(http.MethodGet, targetLink, nil)
		req2.Header.Set("User-Agent", userAgent)
		resp, err = client.Do(req2)
		if err != nil {
			return &database.BrokenLink{
				SourcePage: sourcePage,
				TargetLink: targetLink,
				StatusCode: 0,
				ErrorMsg:   err.Error(),
				LinkType:   "broken",
			}
		}
	}
	defer resp.Body.Close()

	code := resp.StatusCode

	// 405: HEAD not allowed — retry with GET
	if code == http.StatusMethodNotAllowed || code == 501 {
		req2, _ := http.NewRequest(http.MethodGet, targetLink, nil)
		req2.Header.Set("User-Agent", userAgent)
		resp2, err := client.Do(req2)
		if err != nil {
			return &database.BrokenLink{
				SourcePage: sourcePage,
				TargetLink: targetLink,
				StatusCode: 0,
				ErrorMsg:   err.Error(),
				LinkType:   "broken",
			}
		}
		defer resp2.Body.Close()
		code = resp2.StatusCode
	}

	switch {
	case code >= 200 && code < 300:
		return nil // OK
	case code >= 300 && code < 400:
		return &database.BrokenLink{
			SourcePage: sourcePage,
			TargetLink: targetLink,
			StatusCode: code,
			LinkType:   "redirect",
		}
	default:
		return &database.BrokenLink{
			SourcePage: sourcePage,
			TargetLink: targetLink,
			StatusCode: code,
			ErrorMsg:   fmt.Sprintf("HTTP %d", code),
			LinkType:   "broken",
		}
	}
}
