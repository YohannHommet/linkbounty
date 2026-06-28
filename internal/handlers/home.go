package handlers

import "net/http"

func (a *App) handleHome(w http.ResponseWriter, r *http.Request) {
	a.render(w, "home.tmpl", nil)
}
