package handlers

import "net/http"

func (a *App) handle404(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	a.render(w, "notfound.tmpl", nil)
}
