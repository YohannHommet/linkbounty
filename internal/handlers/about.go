package handlers

import "net/http"

func (a *App) handleAbout(w http.ResponseWriter, r *http.Request) {
	a.render(w, "about.tmpl", nil)
}
