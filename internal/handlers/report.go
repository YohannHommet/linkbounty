package handlers

import (
	"net/http"
)

type reportData struct {
	JobID        string
	Domain       string
	Status       string
	ErrorMsg     string
	PagesCrawled int
	BrokenCount  int
	Groups       []groupData
	Redirects    []linkData
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
		a.render(w, "error.tmpl", map[string]string{
			"Message": "Ce rapport n'existe pas ou a expiré (7 jours). Lance un nouveau scan gratuitement.",
		})
		return
	}

	data := reportData{
		JobID:        job.ID,
		Domain:       job.Domain,
		Status:       job.Status,
		ErrorMsg:     job.ErrorMsg,
		PagesCrawled: job.PagesCrawled,
		BrokenCount:  job.BrokenCount,
	}

	if job.Status == "done" {
		raw, err := a.DB.GetReportGroups(id)
		if err != nil {
			http.Error(w, "Erreur interne.", http.StatusInternalServerError)
			return
		}
		for _, g := range raw {
			gd := groupData{SourcePage: g.SourcePage}
			for _, l := range g.Links {
				ld := linkData{
					SourcePage: l.SourcePage,
					TargetLink: l.TargetLink,
					StatusCode: l.StatusCode,
					ErrorMsg:   l.ErrorMsg,
				}
				if l.LinkType == "redirect" {
					data.Redirects = append(data.Redirects, ld)
				} else {
					gd.BrokenLinks = append(gd.BrokenLinks, ld)
				}
			}
			if len(gd.BrokenLinks) > 0 {
				data.Groups = append(data.Groups, gd)
			}
		}
	}

	a.render(w, "report.tmpl", data)
}
