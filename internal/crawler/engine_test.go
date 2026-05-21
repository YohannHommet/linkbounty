package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockSite builds a test HTTP server with internal pages and external links.
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
		"/": fmt.Sprintf(`<html><body><a href="/about">about</a><a href="%s/good">ext</a></body></html>`, ext.URL),
		"/about": `<html><body><p>About us</p></body></html>`,
	})
	defer site.Close()

	links, pages, err := Run(site.URL+"/", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if pages < 1 {
		t.Errorf("pages = %d, want >= 1", pages)
	}

	var broken []string
	for _, l := range links {
		if l.LinkType == "broken" {
			broken = append(broken, l.TargetLink)
		}
	}
	if len(broken) > 0 {
		t.Errorf("expected 0 broken links, got: %v", broken)
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

	links, _, err := Run(site.URL+"/", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var found bool
	for _, l := range links {
		if l.LinkType == "broken" && strings.Contains(l.TargetLink, dead.URL) {
			found = true
		}
	}
	if !found {
		t.Error("expected a broken link to the dead server, found none")
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

	links, _, err := Run(site.URL+"/", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var foundRedirect bool
	for _, l := range links {
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

	links, _, err := Run(site.URL+"/", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, l := range links {
		if l.LinkType == "broken" && strings.Contains(l.TargetLink, ext.URL) {
			t.Errorf("405→GET fallback should succeed, but link marked broken: %+v", l)
		}
	}
	if calls < 2 {
		t.Errorf("expected at least 2 calls (HEAD + GET fallback), got %d", calls)
	}
}

func TestRunProgressCallback(t *testing.T) {
	site := mockSite(t, map[string]string{
		"/": `<html><body><a href="/p1">p1</a><a href="/p2">p2</a></body></html>`,
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
