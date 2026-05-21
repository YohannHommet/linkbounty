package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"linkbounty/internal/crawler"
)

const rateLimitWindow = 5 * time.Minute

var privateRanges = []string{
	"localhost", "127.", "0.", "10.", "192.168.", "172.16.", "172.17.",
	"172.18.", "172.19.", "172.20.", "172.21.", "172.22.", "172.23.",
	"172.24.", "172.25.", "172.26.", "172.27.", "172.28.", "172.29.",
	"172.30.", "172.31.", "169.254.", "::1", "[::1]",
}

func (a *App) handleScan(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.FormValue("url"))
	if rawURL == "" {
		a.renderError(w, r, http.StatusUnprocessableEntity, "L'URL est requise.")
		return
	}

	// normalize: add https:// if no scheme
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		a.renderError(w, r, http.StatusUnprocessableEntity, "URL invalide. Exemple : https://monblog.fr")
		return
	}

	host := strings.ToLower(parsed.Hostname())
	for _, prefix := range privateRanges {
		if strings.HasPrefix(host, prefix) || host == strings.TrimSuffix(prefix, ".") {
			a.renderError(w, r, http.StatusUnprocessableEntity, "Cette URL n'est pas autorisée.")
			return
		}
	}

	ip := clientIP(r)

	a.rateMu.Lock()
	lastScan, seen := a.rateMap[ip]
	if seen && time.Since(lastScan) < rateLimitWindow {
		remaining := rateLimitWindow - time.Since(lastScan)
		a.rateMu.Unlock()
		a.renderError(w, r, http.StatusTooManyRequests,
			fmt.Sprintf("Un scan par IP toutes les 5 minutes. Réessaie dans %ds.", int(remaining.Seconds())))
		return
	}
	a.rateMap[ip] = time.Now()
	a.rateMu.Unlock()

	// global concurrency cap
	select {
	case a.scanSem <- struct{}{}:
	default:
		a.renderError(w, r, http.StatusServiceUnavailable,
			"Le serveur est occupé. Réessaie dans 30 secondes.")
		return
	}

	jobID := uuid.New().String()
	domain := parsed.Hostname()

	if err := a.DB.CreateJob(jobID, domain); err != nil {
		<-a.scanSem
		a.renderError(w, r, http.StatusInternalServerError, "Erreur interne. Réessaie.")
		return
	}

	state := a.Registry.Register(jobID)

	go func() {
		defer func() { <-a.scanSem }()
		defer a.Registry.Delete(jobID)

		links, pages, err := crawler.Run(rawURL, func(n int) {
			state.PagesCrawled.Store(int64(n))
		})
		if err != nil {
			state.Status.Store(StatusError)
			state.ErrorMsg.Store(err.Error())
			a.DB.UpdateJobError(jobID, err.Error())
			return
		}

		brokenCount := 0
		for _, l := range links {
			if l.LinkType == "broken" {
				brokenCount++
			}
		}

		if err := a.DB.SaveLinks(jobID, links); err != nil {
			state.Status.Store(StatusError)
			state.ErrorMsg.Store("Erreur lors de la sauvegarde du rapport.")
			a.DB.UpdateJobError(jobID, "save links: "+err.Error())
			return
		}

		if err := a.DB.UpdateJobDone(jobID, pages, brokenCount); err != nil {
			state.Status.Store(StatusError)
			state.ErrorMsg.Store("Erreur lors de la finalisation du rapport.")
			a.DB.UpdateJobError(jobID, "update done: "+err.Error())
			return
		}

		state.Status.Store(StatusDone)
	}()

	http.Redirect(w, r, "/r/"+jobID, http.StatusFound)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		return ip[:colon]
	}
	return ip
}

func (a *App) renderError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	w.WriteHeader(code)
	// For HTMX requests, return inline error; otherwise render full error page
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<p class="text-red-600 text-sm mt-1">%s</p>`, msg)
		return
	}
	a.render(w, "error.tmpl", map[string]string{"Message": msg})
}
