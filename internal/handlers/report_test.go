package handlers

import (
	"testing"

	"linkbounty/internal/database"
)

func TestBuildReportDataRoutesLinkTypes(t *testing.T) {
	job := &database.Job{
		ID:            "job-1",
		Domain:        "example.com",
		Status:        database.StatusDone,
		PagesCrawled:  20,
		BrokenCount:   1,
		ExtLinksFound: 30,
	}
	groups := []database.ReportGroup{
		{
			SourcePage: "https://example.com/page",
			Links: []database.BrokenLink{
				{SourcePage: "https://example.com/page", TargetLink: "https://dead.test", StatusCode: 404, LinkType: "broken"},
				{SourcePage: "https://example.com/page", TargetLink: "https://moved.test", StatusCode: 301, LinkType: "redirect"},
				{SourcePage: "https://example.com/page", TargetLink: "https://linkedin.test", StatusCode: 999, LinkType: "unverifiable"},
			},
		},
	}

	data := buildReportData(job, groups)

	if data.Status != "done" {
		t.Errorf("Status = %q, want done", data.Status)
	}
	if len(data.Groups) != 1 || len(data.Groups[0].BrokenLinks) != 1 {
		t.Fatalf("want 1 group with 1 broken link, got %+v", data.Groups)
	}
	if len(data.Redirects) != 1 {
		t.Errorf("Redirects = %d, want 1", len(data.Redirects))
	}
	if len(data.Unverifiable) != 1 {
		t.Errorf("Unverifiable = %d, want 1", len(data.Unverifiable))
	}
}

func TestBuildReportDataSPAWarning(t *testing.T) {
	// Few external links after many pages → SPA heads-up.
	job := &database.Job{
		Status:        database.StatusDone,
		PagesCrawled:  spaPageThreshold,
		ExtLinksFound: spaLinkThreshold - 1,
	}
	if !buildReportData(job, nil).SPAWarning {
		t.Error("expected SPAWarning when ext links are below threshold after enough pages")
	}

	// Plenty of external links → no warning.
	job.ExtLinksFound = spaLinkThreshold + 10
	if buildReportData(job, nil).SPAWarning {
		t.Error("did not expect SPAWarning when external links are plentiful")
	}
}

func TestBuildReportDataTruncationFlag(t *testing.T) {
	job := &database.Job{Status: database.StatusDone, ExtLinksFound: extLinkCap + 1}
	if !buildReportData(job, nil).ExtLinksTruncated {
		t.Error("expected ExtLinksTruncated when ext links exceed the cap")
	}

	job.ExtLinksFound = extLinkCap
	if buildReportData(job, nil).ExtLinksTruncated {
		t.Error("did not expect truncation flag at exactly the cap")
	}
}

func TestBuildReportDataDropsEmptyGroups(t *testing.T) {
	// A group with only a redirect (no broken links) must not appear in Groups.
	job := &database.Job{Status: database.StatusDone}
	groups := []database.ReportGroup{
		{
			SourcePage: "https://example.com/p",
			Links: []database.BrokenLink{
				{TargetLink: "https://moved.test", StatusCode: 302, LinkType: "redirect"},
			},
		},
	}
	data := buildReportData(job, groups)
	if len(data.Groups) != 0 {
		t.Errorf("Groups = %d, want 0 (redirect-only group should be dropped)", len(data.Groups))
	}
	if len(data.Redirects) != 1 {
		t.Errorf("Redirects = %d, want 1", len(data.Redirects))
	}
}
