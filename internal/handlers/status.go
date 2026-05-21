package handlers

import (
	"fmt"
	"net/http"
)

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("uuid")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Try registry first (job is in-flight)
	if state, ok := a.Registry.Get(id); ok {
		status := state.Status.Load().(JobStatus)
		switch status {
		case StatusRunning:
			pages := state.PagesCrawled.Load()
			fmt.Fprintf(w, `<div id="progress"
				hx-get="/r/%s/status"
				hx-trigger="every 2s"
				hx-swap="outerHTML"
				aria-live="polite">
				<div class="flex items-center gap-3">
					<div class="animate-spin h-4 w-4 border-2 border-[var(--color-accent)] border-t-transparent rounded-full"></div>
					<span>Scan en cours… <strong>%d</strong> pages analysées</span>
				</div>
			</div>`, id, pages)
		case StatusDone:
			fmt.Fprintf(w, `<div id="progress"
				hx-get="/r/%s"
				hx-trigger="load"
				hx-swap="outerHTML"
				hx-target="body">
				Chargement du rapport…
			</div>`, id)
		case StatusError:
			msg := state.ErrorMsg.Load().(string)
			fmt.Fprintf(w, `<div id="progress" aria-live="polite">
				<p class="text-[var(--color-error)]">Erreur : %s. <a href="/" class="underline">Réessayer</a></p>
			</div>`, msg)
		}
		return
	}

	// Registry miss — job may have completed before this poll or server restarted
	// Fall back to SQLite
	job, err := a.DB.GetJob(id)
	if err != nil || job == nil {
		fmt.Fprintf(w, `<div id="progress">
			<p class="text-[var(--color-error)]">Rapport introuvable. <a href="/" class="underline">Nouveau scan</a></p>
		</div>`)
		return
	}

	switch job.Status {
	case "done":
		fmt.Fprintf(w, `<div id="progress"
			hx-get="/r/%s"
			hx-trigger="load"
			hx-swap="outerHTML"
			hx-target="body">
			Chargement du rapport…
		</div>`, id)
	case "error":
		fmt.Fprintf(w, `<div id="progress" aria-live="polite">
			<p class="text-[var(--color-error)]">Erreur : %s. <a href="/" class="underline">Réessayer</a></p>
		</div>`, job.ErrorMsg)
	default:
		// status=running but not in registry — server restarted mid-crawl, treat as error
		fmt.Fprintf(w, `<div id="progress" aria-live="polite">
			<p class="text-[var(--color-error)]">Le scan a été interrompu. <a href="/" class="underline">Réessayer</a></p>
		</div>`)
	}
}
