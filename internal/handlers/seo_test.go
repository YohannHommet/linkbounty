package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestStaticAndSEORoutesStatus verifies that the router returns the expected
// HTTP status code for each well-known path, including the catch-all 404.
func TestStaticAndSEORoutesStatus(t *testing.T) {
	app := newTestApp(t)
	handler := app.Routes()

	cases := []struct {
		path       string
		wantStatus int
	}{
		{"/", http.StatusOK},
		{"/a-propos", http.StatusOK},
		{"/bot", http.StatusOK},
		{"/confidentialite", http.StatusOK},
		{"/favicon.ico", http.StatusMovedPermanently},
		{"/static/favicon.svg", http.StatusOK},
		{"/this-does-not-exist", http.StatusNotFound},
		{"/r/00000000-0000-0000-0000-000000000000", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Errorf("GET %s: status = %d, want %d", tc.path, w.Code, tc.wantStatus)
			}
		})
	}
}

// TestRobotsTxt verifies that /robots.txt is served as text/plain and contains
// the required directives.
func TestRobotsTxt(t *testing.T) {
	app := newTestApp(t)
	handler := app.Routes()

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /robots.txt: status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Disallow: /r/") {
		t.Errorf("robots.txt body missing \"Disallow: /r/\"; got:\n%s", body)
	}
	if !strings.Contains(body, "Sitemap:") {
		t.Errorf("robots.txt body missing \"Sitemap:\"; got:\n%s", body)
	}
}

// TestSitemapXML verifies that /sitemap.xml returns 200 and contains the
// expected URL entries.
func TestSitemapXML(t *testing.T) {
	app := newTestApp(t)
	handler := app.Routes()

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /sitemap.xml: status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<loc>") {
		t.Errorf("sitemap.xml body missing \"<loc>\"; got:\n%s", body)
	}
	if !strings.Contains(body, "/a-propos") {
		t.Errorf("sitemap.xml body missing \"/a-propos\"; got:\n%s", body)
	}
}

// TestSecurityHeaders verifies that the security middleware injects the
// expected headers on every response.
func TestSecurityHeaders(t *testing.T) {
	app := newTestApp(t)
	handler := app.Routes()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want \"nosniff\"", got)
	}

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header is missing or empty")
	}
}
