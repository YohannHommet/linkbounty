package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"
	"sync"
	"time"

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
	rateMu   sync.Mutex
	rateMap  map[string]time.Time // IP → last scan time
}

func NewApp(db *database.Store, registry *JobRegistry, uiDir string) (*App, error) {
	a := &App{
		DB:       db,
		Registry: registry,
		tmpls:    make(map[string]*template.Template),
		scanSem:  make(chan struct{}, 10),
		rateMap:  make(map[string]time.Time),
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

func (a *App) renderFragment(w http.ResponseWriter, name string, tmplName string, data any) {
	ts, ok := a.tmpls[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ts.ExecuteTemplate(w, tmplName, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleHome)
	mux.HandleFunc("POST /scan", a.handleScan)
	mux.HandleFunc("GET /r/{uuid}", a.handleReport)
	mux.HandleFunc("GET /r/{uuid}/status", a.handleStatus)
	mux.HandleFunc("GET /robots.txt", a.handleRobots)
	return mux
}
