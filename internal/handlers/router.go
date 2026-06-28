package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"

	"linkbounty/internal/database"
)

var funcMap = template.FuncMap{
	"truncate": func(s string, n int) string {
		runes := []rune(s)
		if len(runes) <= n {
			return s
		}
		return string(runes[:n]) + "…"
	},
}

type App struct {
	DB       *database.Store
	Registry *JobRegistry
	tmpls    map[string]*template.Template
	scanSem  chan struct{} // global concurrency: max 10 simultaneous scans
	rate     *RateLimiter
	uiDir    string
}

func NewApp(db *database.Store, registry *JobRegistry, uiDir string) (*App, error) {
	a := &App{
		DB:       db,
		Registry: registry,
		tmpls:    make(map[string]*template.Template),
		scanSem:  make(chan struct{}, 10),
		rate:     NewRateLimiter(rateLimitWindow),
		uiDir:    uiDir,
	}
	if err := a.compileTemplates(uiDir); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) compileTemplates(uiDir string) error {
	base := filepath.Join(uiDir, "layouts", "base.tmpl")
	components, err := filepath.Glob(filepath.Join(uiDir, "components", "*.tmpl"))
	if err != nil {
		return err
	}

	pages, err := filepath.Glob(filepath.Join(uiDir, "pages", "*.tmpl"))
	if err != nil {
		return err
	}

	for _, page := range pages {
		files := append([]string{base}, components...)
		files = append(files, page)

		ts, err := template.New(filepath.Base(page)).Funcs(funcMap).ParseFiles(files...)
		if err != nil {
			return err
		}
		a.tmpls[filepath.Base(page)] = ts
	}
	return nil
}

func (a *App) render(w http.ResponseWriter, name string, data any) {
	ts, ok := a.tmpls[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ts.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.handleHome)
	mux.HandleFunc("GET /a-propos", a.handleAbout)
	mux.HandleFunc("POST /scan", a.handleScan)
	mux.HandleFunc("GET /r/{uuid}", a.handleReport)
	mux.HandleFunc("GET /r/{uuid}/status", a.handleStatus)
	mux.HandleFunc("GET /r/{uuid}/og.png", a.handleReportOG)
	mux.HandleFunc("GET /robots.txt", a.handleRobots)
	mux.HandleFunc("GET /sitemap.xml", a.handleSitemap)
	mux.HandleFunc("GET /bot", a.handleBot)
	mux.HandleFunc("GET /confidentialite", a.handlePrivacy)

	// Static assets (favicon, OG image, app icons, manifest).
	staticFS := http.FileServer(http.Dir(filepath.Join(a.uiDir, "static")))
	mux.Handle("GET /static/", cacheControl(http.StripPrefix("/static/", staticFS)))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/favicon.svg", http.StatusMovedPermanently)
	})

	// Catch-all: any path not matched above returns a proper 404 page.
	mux.HandleFunc("GET /", a.handle404)

	return securityHeaders(accessLog(mux))
}

// cacheControl adds a long max-age to static assets (they are content-stable).
func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}
