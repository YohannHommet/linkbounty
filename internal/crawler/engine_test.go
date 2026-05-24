package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mockSite(t *testing.T, pages map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, body := range pages {
		p, b := path, body
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, b)
		})
	}
	return httptest.NewServer(mux)
}

func TestRunFindsNoBrokenLinksOnCleanSite(t *testing.T) {
	ext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ext.Close()

	site := mockSite(t, map[string]string{
		"/":      fmt.Sprintf(`<html><body><a href="/about">about</a><a href="%s/good">ext</a></body></html>`, ext.URL),
		"/about": `<html><body><p>About us</p></body></html>`,
	})
	defer site.Close()

	result, err := Run(site.URL+"/", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.PagesCrawled < 1 {
		t.Errorf("PagesCrawled = %d, want >= 1", result.PagesCrawled)
	}
	if result.ExtLinksFound < 1 {
		t.Errorf("ExtLinksFound = %d, want >= 1", result.ExtLinksFound)
	}
	for _, l := range result.Links {
		if l.LinkType == "broken" {
			t.Errorf("expected 0 broken links, got: %+v", l)
		}
	}
}

func TestRunDetectsBrokenExternalLinks(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer dead.Close()

	site := mockSite(t, map[string]string{
		"/": fmt.Sprintf(`<html><body><a href="%s/missing">dead link</a></body></html>`, dead.URL),
	})
	defer site.Close()

	result, err := Run(site.URL+"/", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var found bool
	for _, l := range result.Links {
		if l.LinkType == "broken" && strings.Contains(l.TargetLink, dead.URL) {
			found = true
		}
	}
	if !found {
		t.Error("expected a broken link to the dead server, found none")
	}
	if result.ExtLinksFound < 1 {
		t.Errorf("ExtLinksFound = %d, want >= 1", result.ExtLinksFound)
	}
}

func TestRunDetectsRedirects(t *testing.T) {
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com", http.StatusMovedPermanently)
	}))
	defer redirect.Close()

	site := mockSite(t, map[string]string{
		"/": fmt.Sprintf(`<html><body><a href="%s/">redirect</a></body></html>`, redirect.URL),
	})
	defer site.Close()

	result, err := Run(site.URL+"/", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var foundRedirect bool
	for _, l := range result.Links {
		if l.LinkType == "redirect" {
			foundRedirect = true
		}
	}
	if !foundRedirect {
		t.Error("expected a redirect link, found none")
	}
}

func TestRunFallsBackToGETOn405(t *testing.T) {
	calls := 0
	ext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ext.Close()

	site := mockSite(t, map[string]string{
		"/": fmt.Sprintf(`<html><body><a href="%s/only-get">ext</a></body></html>`, ext.URL),
	})
	defer site.Close()

	result, err := Run(site.URL+"/", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, l := range result.Links {
		if l.LinkType == "broken" && strings.Contains(l.TargetLink, ext.URL) {
			t.Errorf("405→GET fallback should succeed, but link marked broken: %+v", l)
		}
	}
	if calls < 2 {
		t.Errorf("expected at least 2 calls (HEAD + GET fallback), got %d", calls)
	}
}

func TestRunExtLinksFoundCount(t *testing.T) {
	ext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ext.Close()

	// 3 distinct external links
	site := mockSite(t, map[string]string{
		"/": fmt.Sprintf(`<html><body>
			<a href="%s/a">a</a>
			<a href="%s/b">b</a>
			<a href="%s/c">c</a>
		</body></html>`, ext.URL, ext.URL, ext.URL),
	})
	defer site.Close()

	result, err := Run(site.URL+"/", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExtLinksFound != 3 {
		t.Errorf("ExtLinksFound = %d, want 3", result.ExtLinksFound)
	}
}

func TestRunProgressCallback(t *testing.T) {
	site := mockSite(t, map[string]string{
		"/":   `<html><body><a href="/p1">p1</a><a href="/p2">p2</a></body></html>`,
		"/p1": `<html><body><p>page 1</p></body></html>`,
		"/p2": `<html><body><p>page 2</p></body></html>`,
	})
	defer site.Close()

	var maxSeen int
	Run(site.URL+"/", func(n int) {
		if n > maxSeen {
			maxSeen = n
		}
	})
	if maxSeen < 1 {
		t.Errorf("progress callback never fired or reported 0 pages")
	}
}
