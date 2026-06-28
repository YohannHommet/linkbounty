package handlers

import "net/http"

func (a *App) handleBot(w http.ResponseWriter, r *http.Request) {
	a.render(w, "bot.tmpl", nil)
}
