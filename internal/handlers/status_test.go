package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"linkbounty/internal/database"
)

func TestStatusRunningFromRegistry(t *testing.T) {
	app := newTestApp(t)
	state := app.Registry.Register("test-running")
	state.SetStatus(database.StatusRunning)
	state.PagesCrawled.Store(12)

	req := httptest.NewRequest(http.MethodGet, "/r/test-running/status", nil)
	req.SetPathValue("uuid", "test-running")
	w := httptest.NewRecorder()

	app.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "hx-trigger") {
		t.Error("running fragment should contain hx-trigger for polling")
	}
	if !strings.Contains(body, "12") {
		t.Error("running fragment should show page count")
	}
}

func TestStatusDoneFromRegistry(t *testing.T) {
	app := newTestApp(t)
	state := app.Registry.Register("test-done")
	state.SetStatus(database.StatusDone)

	req := httptest.NewRequest(http.MethodGet, "/r/test-done/status", nil)
	req.SetPathValue("uuid", "test-done")
	w := httptest.NewRecorder()

	app.handleStatus(w, req)

	body := w.Body.String()
	if strings.Contains(body, `hx-trigger="every 2s"`) {
		t.Error("done fragment must NOT have polling trigger")
	}
	if !strings.Contains(body, "hx-get") {
		t.Error("done fragment should redirect to report")
	}
}

func TestStatusErrorFromRegistry(t *testing.T) {
	app := newTestApp(t)
	state := app.Registry.Register("test-err")
	state.SetStatus(database.StatusError)
	state.SetErrorMsg("connection refused")

	req := httptest.NewRequest(http.MethodGet, "/r/test-err/status", nil)
	req.SetPathValue("uuid", "test-err")
	w := httptest.NewRecorder()

	app.handleStatus(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "connection refused") {
		t.Error("error fragment should show the error message")
	}
}

func TestStatusFallsBackToSQLiteOnRegistryMiss(t *testing.T) {
	db, _ := database.New(":memory:")
	app, _ := NewApp(db, &JobRegistry{}, "../../ui/html")

	db.CreateJob("sqlite-job", "example.com")
	db.UpdateJobDone("sqlite-job", 10, 2, 45)

	req := httptest.NewRequest(http.MethodGet, "/r/sqlite-job/status", nil)
	req.SetPathValue("uuid", "sqlite-job")
	w := httptest.NewRecorder()

	app.handleStatus(w, req)

	body := w.Body.String()
	// done job not in registry → should redirect to report
	if !strings.Contains(body, "hx-get") {
		t.Error("SQLite fallback for done job should return redirect fragment")
	}
}

func TestStatusReturns404FragmentForUnknownJob(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/r/ghost/status", nil)
	req.SetPathValue("uuid", "ghost")
	w := httptest.NewRecorder()

	app.handleStatus(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "introuvable") {
		t.Error("unknown job should return an error fragment")
	}
}
