package handlers

import (
	"fmt"
	"html"
	"net/http"

	"linkbounty/internal/database"
)

const (
	fragDone = `<div id="progress"
		hx-get="/r/%s"
		hx-trigger="load"
		hx-swap="outerHTML"
		hx-target="body"
		class="scan-wait">
		<p class="swiss-status">Chargement du rapport…</p>
	</div>`

	fragRetry = `<div id="progress" aria-live="polite" class="scan-wait">
		<p style="font-size:13px;font-weight:600;color:var(--signal);text-align:center;max-width:520px;line-height:1.5;">%s</p>
		<a href="/" class="btn-outline">%s</a>
	</div>`
)

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("uuid")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Try registry first (job is in-flight)
	if state, ok := a.Registry.Get(id); ok {
		switch state.GetStatus() {
		case database.StatusRunning:
			pages := state.PagesCrawled.Load()
			fmt.Fprintf(w, `<div id="progress"
				hx-get="/r/%s/status"
				hx-trigger="every 2s"
				hx-swap="outerHTML"
				aria-live="polite"
				class="scan-wait">
				<div class="swiss-loader" aria-hidden="true"><span></span></div>
				<p class="swiss-status">Scan en cours — <strong>%d</strong> pages analysées</p>
			</div>`, id, pages)
		case database.StatusDone:
			fmt.Fprintf(w, fragDone, id)
		case database.StatusError:
			fmt.Fprintf(w, fragRetry, "Erreur : "+html.EscapeString(state.GetErrorMsg()), "Réessayer")
		}
		return
	}

	// Registry miss — job may have completed before this poll or server restarted.
	// Fall back to SQLite.
	job, err := a.DB.GetJob(id)
	if err != nil || job == nil {
		fmt.Fprintf(w, fragRetry, "Rapport introuvable.", "Nouveau scan")
		return
	}

	switch job.Status {
	case database.StatusDone:
		fmt.Fprintf(w, fragDone, id)
	case database.StatusError:
		fmt.Fprintf(w, fragRetry, "Erreur : "+html.EscapeString(job.ErrorMsg), "Réessayer")
	default:
		// status=running but not in registry — server restarted mid-crawl, treat as error
		fmt.Fprintf(w, fragRetry, "Le scan a été interrompu.", "Réessayer")
	}
}
