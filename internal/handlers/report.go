package handlers

import (
	"net/http"

	"linkbounty/internal/database"
)

const (
	spaLinkThreshold = 5  // fewer external links than this after ≥spaPageThreshold pages → likely SPA
	spaPageThreshold = 10 // need at least this many pages crawled before the SPA signal is meaningful
	extLinkCap       = 500
)

type reportData struct {
	JobID             string
	Domain            string
	Status            string
	ErrorMsg          string
	PagesCrawled      int
	BrokenCount       int
	ExtLinksFound     int
	ExtLinksTruncated bool // true when >500 external links were found
	SPAWarning        bool // true when site appears to use client-side rendering
	Groups            []groupData
	Redirects         []linkData
	Unverifiable      []linkData
}

type groupData struct {
	SourcePage  string
	BrokenLinks []linkData
}

type linkData struct {
	SourcePage string
	TargetLink string
	StatusCode int
	ErrorMsg   string
}

func (a *App) handleReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("uuid")

	job, err := a.DB.GetJob(id)
	if err != nil {
		http.Error(w, "Erreur interne.", http.StatusInternalServerError)
		return
	}
	if job == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		a.render(w, "error.tmpl", map[string]string{
			"Message": "Ce rapport n'existe pas ou a expiré (7 jours). Lance un nouveau scan gratuitement.",
		})
		return
	}

	var groups []database.ReportGroup
	if job.Status == database.StatusDone {
		groups, err = a.DB.GetReportGroups(id)
		if err != nil {
			http.Error(w, "Erreur interne.", http.StatusInternalServerError)
			return
		}
	}

	a.render(w, "report.tmpl", buildReportData(job, groups))
}

// buildReportData is a pure function mapping DB records to template data.
func buildReportData(job *database.Job, groups []database.ReportGroup) reportData {
	data := reportData{
		JobID:             job.ID,
		Domain:            job.Domain,
		Status:            string(job.Status),
		ErrorMsg:          job.ErrorMsg,
		PagesCrawled:      job.PagesCrawled,
		BrokenCount:       job.BrokenCount,
		ExtLinksFound:     job.ExtLinksFound,
		ExtLinksTruncated: job.ExtLinksFound > extLinkCap,
		SPAWarning:        job.ExtLinksFound < spaLinkThreshold && job.PagesCrawled >= spaPageThreshold,
	}

	for _, g := range groups {
		gd := groupData{SourcePage: g.SourcePage}
		for _, l := range g.Links {
			ld := linkData{
				SourcePage: l.SourcePage,
				TargetLink: l.TargetLink,
				StatusCode: l.StatusCode,
				ErrorMsg:   l.ErrorMsg,
			}
			switch l.LinkType {
			case "redirect":
				data.Redirects = append(data.Redirects, ld)
			case "unverifiable":
				data.Unverifiable = append(data.Unverifiable, ld)
			default:
				gd.BrokenLinks = append(gd.BrokenLinks, ld)
			}
		}
		if len(gd.BrokenLinks) > 0 {
			data.Groups = append(data.Groups, gd)
		}
	}

	return data
}
