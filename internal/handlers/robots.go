package handlers

import (
	"fmt"
	"net/http"
)

func (a *App) handleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, `User-agent: *
Allow: /$
Disallow: /r/
Disallow: /scan

Sitemap: %s/sitemap.xml
`, siteURL())
}
