package database

import (
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.db.Close() })
	return s
}

func TestCreateAndGetJob(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateJob("job-1", "example.com"); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	job, err := s.GetJob("job-1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job == nil {
		t.Fatal("expected job, got nil")
	}
	if job.Domain != "example.com" {
		t.Errorf("domain = %q, want %q", job.Domain, "example.com")
	}
	if job.Status != "running" {
		t.Errorf("status = %q, want running", job.Status)
	}
}

func TestGetJobNotFound(t *testing.T) {
	s := newTestStore(t)
	job, err := s.GetJob("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job != nil {
		t.Errorf("expected nil, got %+v", job)
	}
}

func TestUpdateJobDone(t *testing.T) {
	s := newTestStore(t)
	s.CreateJob("job-2", "blog.com")

	if err := s.UpdateJobDone("job-2", 47, 3); err != nil {
		t.Fatalf("UpdateJobDone: %v", err)
	}

	job, _ := s.GetJob("job-2")
	if job.Status != "done" {
		t.Errorf("status = %q, want done", job.Status)
	}
	if job.PagesCrawled != 47 {
		t.Errorf("pages_crawled = %d, want 47", job.PagesCrawled)
	}
	if job.BrokenCount != 3 {
		t.Errorf("broken_count = %d, want 3", job.BrokenCount)
	}
}

func TestUpdateJobError(t *testing.T) {
	s := newTestStore(t)
	s.CreateJob("job-3", "broken.com")

	if err := s.UpdateJobError("job-3", "DNS failure"); err != nil {
		t.Fatalf("UpdateJobError: %v", err)
	}

	job, _ := s.GetJob("job-3")
	if job.Status != "error" {
		t.Errorf("status = %q, want error", job.Status)
	}
	if job.ErrorMsg != "DNS failure" {
		t.Errorf("error_msg = %q, want 'DNS failure'", job.ErrorMsg)
	}
}

func TestMarkOrphansError(t *testing.T) {
	s := newTestStore(t)
	s.CreateJob("orphan-1", "a.com")
	s.CreateJob("orphan-2", "b.com")
	s.UpdateJobDone("orphan-2", 10, 0)

	if err := s.MarkOrphansError(); err != nil {
		t.Fatalf("MarkOrphansError: %v", err)
	}

	j1, _ := s.GetJob("orphan-1")
	if j1.Status != "error" {
		t.Errorf("orphan-1 status = %q, want error", j1.Status)
	}

	j2, _ := s.GetJob("orphan-2")
	if j2.Status != "done" {
		t.Errorf("orphan-2 should stay done, got %q", j2.Status)
	}
}

func TestSaveLinksAndGetReportGroups(t *testing.T) {
	s := newTestStore(t)
	s.CreateJob("job-r", "site.com")

	links := []BrokenLink{
		{SourcePage: "https://site.com/page-a", TargetLink: "https://dead.com/1", StatusCode: 404, ErrorMsg: "HTTP 404", LinkType: "broken"},
		{SourcePage: "https://site.com/page-a", TargetLink: "https://dead.com/2", StatusCode: 500, ErrorMsg: "HTTP 500", LinkType: "broken"},
		{SourcePage: "https://site.com/page-b", TargetLink: "https://moved.com/x", StatusCode: 301, LinkType: "redirect"},
	}

	if err := s.SaveLinks("job-r", links); err != nil {
		t.Fatalf("SaveLinks: %v", err)
	}

	groups, err := s.GetReportGroups("job-r")
	if err != nil {
		t.Fatalf("GetReportGroups: %v", err)
	}

	var brokenCount, redirectCount int
	for _, g := range groups {
		for _, l := range g.Links {
			if l.LinkType == "broken" {
				brokenCount++
			} else {
				redirectCount++
			}
		}
	}
	if brokenCount != 2 {
		t.Errorf("broken links = %d, want 2", brokenCount)
	}
	if redirectCount != 1 {
		t.Errorf("redirects = %d, want 1", redirectCount)
	}
}

func TestSaveLinksEmpty(t *testing.T) {
	s := newTestStore(t)
	s.CreateJob("job-empty", "clean.com")

	if err := s.SaveLinks("job-empty", nil); err != nil {
		t.Errorf("SaveLinks with nil should be no-op, got: %v", err)
	}
	if err := s.SaveLinks("job-empty", []BrokenLink{}); err != nil {
		t.Errorf("SaveLinks with empty slice should be no-op, got: %v", err)
	}
}

func TestCleanupCascade(t *testing.T) {
	s := newTestStore(t)
	s.CreateJob("old-job", "old.com")
	s.SaveLinks("old-job", []BrokenLink{
		{SourcePage: "https://old.com/p", TargetLink: "https://dead.com", StatusCode: 404, LinkType: "broken"},
	})
	s.UpdateJobDone("old-job", 1, 1)

	if err := s.Cleanup(0); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	job, _ := s.GetJob("old-job")
	if job != nil {
		t.Error("expected job to be deleted by cleanup, but it still exists")
	}

	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM broken_links WHERE job_id='old-job'`).Scan(&count)
	if count != 0 {
		t.Errorf("expected broken_links cascade deleted, got %d rows", count)
	}
}

func TestCleanupPreservesRecent(t *testing.T) {
	s := newTestStore(t)
	s.CreateJob("fresh-job", "fresh.com")
	s.UpdateJobDone("fresh-job", 5, 0)

	if err := s.Cleanup(7 * 24 * time.Hour); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	job, _ := s.GetJob("fresh-job")
	if job == nil {
		t.Error("cleanup removed a fresh job that should be preserved")
	}
}
