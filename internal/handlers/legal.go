package handlers

import "net/http"

func (a *App) handlePrivacy(w http.ResponseWriter, r *http.Request) {
	a.render(w, "confidentialite.tmpl", nil)
}
