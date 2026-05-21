package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"linkbounty/internal/database"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() {})

	app, err := NewApp(db, &JobRegistry{}, "../../ui/html")
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	return app
}

func TestScanRejectsEmptyURL(t *testing.T) {
	app := newTestApp(t)

	form := url.Values{"url": {""}}
	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	app.handleScan(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestScanRejectsInvalidURL(t *testing.T) {
	app := newTestApp(t)

	cases := []string{"not-a-url", "ftp://example.com", "javascript:alert(1)"}
	for i, u := range cases {
		form := url.Values{"url": {u}}
		req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = fmt.Sprintf("1.2.3.%d:9000", i+10) // unique IP per case
		w := httptest.NewRecorder()
		app.handleScan(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("url %q: status = %d, want 422", u, w.Code)
		}
	}
}

func TestScanRejectsPrivateAddresses(t *testing.T) {
	app := newTestApp(t)

	cases := []string{
		"http://localhost/",
		"http://127.0.0.1/",
		"http://169.254.1.1/",
		"http://192.168.1.1/",
	}
	for _, u := range cases {
		form := url.Values{"url": {u}}
		req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleScan(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("url %q: status = %d, want 422", u, w.Code)
		}
	}
}

func TestScanAcceptsValidURL(t *testing.T) {
	app := newTestApp(t)

	form := url.Values{"url": {"https://example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()

	app.handleScan(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want 302", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/r/") {
		t.Errorf("redirect to %q, want /r/<uuid>", loc)
	}
}

func TestScanNormalizesURLWithoutScheme(t *testing.T) {
	app := newTestApp(t)

	form := url.Values{"url": {"example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "1.2.3.5:1234"
	w := httptest.NewRecorder()

	app.handleScan(w, req)

	// should succeed (normalizes to https://example.com)
	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want 302 (url without scheme should be accepted)", w.Code)
	}
}

func TestRateLimiterConcurrent(t *testing.T) {
	app := newTestApp(t)

	var accepted, rejected atomic.Int64
	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			form := url.Values{"url": {"https://example.com"}}
			req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.RemoteAddr = "10.0.0.1:9999" // same IP for all
			w := httptest.NewRecorder()
			app.handleScan(w, req)

			switch w.Code {
			case http.StatusFound:
				accepted.Add(1)
			case http.StatusTooManyRequests:
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()

	if accepted.Load() != 1 {
		t.Errorf("accepted = %d, want exactly 1 (rate limiter should allow only one per IP)", accepted.Load())
	}
	if rejected.Load() != goroutines-1 {
		t.Errorf("rejected = %d, want %d", rejected.Load(), goroutines-1)
	}
}

func TestGlobalSemaphoreRejects(t *testing.T) {
	app := newTestApp(t)
	// fill the semaphore
	for i := 0; i < cap(app.scanSem); i++ {
		app.scanSem <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(app.scanSem); i++ {
			<-app.scanSem
		}
	}()

	form := url.Values{"url": {"https://example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "9.9.9.9:1"
	w := httptest.NewRecorder()

	app.handleScan(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when semaphore is full", w.Code)
	}
}
