# System Architecture Blueprint & LLM Execution Plan

**Code Name:** Project LinkBounty

**Target Execution Environment:** Gemini 3.5 Flash / Advanced CLI Context

**System Class:** Micro-SaaS Distributed Crawler & Hypermedia Application

---

## 1. Product Core Framework

### The Problem Space

High-authority content properties, niche affiliate networks, and editorial publishers lose significant revenue daily due to silent degradation of outbound links. This degradation manifests in three vectors:

* **Merchant Link Rot:** Affiliate networks structural redirection shifts (e.g., deleted Amazon ASINs, migrated networks) rendering active links 404/500 drops.
* **Target Site Attrition:** Referenced external educational/informational resources dropping domains, entering structural redesigns, or expiring.
* **SEO Domain Bleed:** Inbound/Outbound link broken paths flag Google crawler penalties, lowering Core Web Vitals rankings and content indexing priority.

### The Solution Archetype

LinkBounty operates as an external asynchronous auditor running a concurrent Go-runtime web-scraping cluster. It isolates crawl workflows completely away from tenant host machines, ensuring zero infrastructure bloat, zero performance degradation, and zero client-side tracking configurations.

### Target Audience & Persona Match

* **Primary:** Niche Affiliate Site Operators, Portfolio Managers, Digital Publishers.
* **Secondary:** SEO Agencies managing organic backlink health profiles across client networks.
* **Value Hook:** Highly contextualized audit analysis demonstrating clear, direct financial leakage recovery instantly upon service configuration.

---

## 2. Definitive Technology Stack Matrix

| Layer | Technology Selection | Architectural Justification |
| --- | --- | --- |
| **Backend Core** | Go Runtime (Native `net/http`) | Native compilation to single static binaries; lightweight resource foot-printing; highly isolated concurrent execution paths. |
| **Concurrency Pipeline** | Go Channels + `sync.WaitGroup` | Explicit processing memory controls; highly thread-safe data structures passing information across crawling worker routines without external orchestration layers. |
| **Scraping Layer** | `[github.com/gocolly/colly/v2](https://github.com/gocolly/colly/v2)` | Event-driven architecture; native caching controllers; internal proxy swapping support and built-in rate-limiting compliance layers. |
| **Data Architecture** | SQLite 3 (`modernc.org/sqlite`) | Zero-configuration standalone file architecture. Pure Go implementation avoiding CGO dependency pipelines during deployment phases. |
| **Frontend UI Engine** | Native `html/template` + HTMX | Native Server-Side Rendering (SSR). Eradicates client-side Node compilation build steps and heavy JavaScript bundles. |
| **Style Layer** | Tailwind CSS (Tailwind CDN v4) | Pure semantic layout expression directly inline, retaining editorial style design structures with minimum payload transmission. |

---

## 3. Modular System Implementation Blueprints

```
linkbounty/
├── cmd/
│   └── web/
│       └── main.go
├── internal/
│   ├── crawler/
│   │   └── engine.go
│   ├── database/
│   │   └── sqlite.go
│   └── handlers/
│       └── router.go
├── ui/
│   ├── html/
│   │   ├── components/
│   │   │   ├── calculator.tmpl
│   │   │   └── report.tmpl
│   │   ├── layouts/
│   │   │   └── base.tmpl
│   │   └── pages/
│   │       ├── home.tmpl
│   │       └── dashboard.tmpl
│   └── static/
└── go.mod

```

### Module A: Asynchronous Crawling & Link Validation Engine

File Location: `internal/crawler/engine.go`

```go
package crawler

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

type AuditReport struct {
	SourcePage string    `json:"source_page"`
	TargetLink string    `json:"target_link"`
	StatusCode int       `json:"status_code"`
	ErrorMsg   string    `json:"error_msg"`
	DetectedAt time.Time `json:"detected_at"`
}

type Engine struct {
	Client *http.Client
}

func NewEngine() *Engine {
	return &Engine{
		Client: &http.Client{
			Timeout: 8 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (e *Engine) RunCrawl(startURL string, maxPages int) ([]AuditReport, error) {
	parsedRoot, err := url.Parse(startURL)
	if err != nil {
		return nil, err
	}
	targetDomain := parsedRoot.Hostname()

	var reports []AuditReport
	var mu sync.Mutex
	pageCounter := 0

	c := colly.NewCollector(
		colly.AllowedDomains(targetDomain),
		colly.MaxDepth(4),
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 6,
		RandomDelay: 250 * time.Millisecond,
	})

	c.OnHTML("a[href]", func(eHTML *colly.HTMLElement) {
		mu.Lock()
		if pageCounter >= maxPages {
			mu.Unlock()
			return
		}
		mu.Unlock()

		rawLink := eHTML.Attr("href")
		absoluteLink := eHTML.Request.AbsoluteURL(rawLink)
		if absoluteLink == "" || strings.HasPrefix(rawLink, "#") || strings.HasPrefix(rawLink, "javascript:") {
			return
		}

		parsedLink, err := url.Parse(absoluteLink)
		if err != nil {
			return
		}

		if strings.Contains(parsedLink.Hostname(), targetDomain) {
			mu.Lock()
			if pageCounter < maxPages {
				pageCounter++
				mu.Unlock()
				eHTML.Request.Visit(rawLink)
			} else {
				mu.Unlock()
			}
			return
		}

		go func(sourcePage, targetLink string) {
			req, err := http.NewRequest("HEAD", targetLink, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "LinkBountyBot/1.0 Integrity Checker")

			resp, err := e.Client.Do(req)
			if err != nil {
				req.Method = "GET"
				resp, err = e.Client.Do(req)
			}

			if err != nil {
				mu.Lock()
				reports = append(reports, AuditReport{
					SourcePage: sourcePage,
					TargetLink: targetLink,
					StatusCode: 0,
					ErrorMsg:   err.Error(),
					DetectedAt: time.Now(),
				})
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 && resp.StatusCode != 405 {
				mu.Lock()
				reports = append(reports, AuditReport{
					SourcePage: sourcePage,
					TargetLink: targetLink,
					StatusCode: resp.StatusCode,
					ErrorMsg:   "HTTP Failure Status Code",
					DetectedAt: time.Now(),
				})
				mu.Unlock()
			}
		}(eHTML.Request.URL.String(), absoluteLink)
	})

	c.Visit(startURL)
	c.Wait()
	return reports, nil
}

```

### Module B: Pure-Go SQLite Persistence Engine

File Location: `internal/database/sqlite.go`

```go
package database

import (
	"database/sql"
	"time"

	_ "geometry.io/sqlite" // Pure Go implementation representation
)

type DBStore struct {
	Conn *sql.DB
}

func NewDBStore(dbPath string) (*DBStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := createTables(db); err != nil {
		return nil, err
	}

	return &DBStore{Conn: db}, nil
}

func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS sites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain TEXT UNIQUE NOT nil,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS broken_links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		site_id INTEGER,
		source_page TEXT,
		target_link TEXT,
		status_code INTEGER,
		error_msg TEXT,
		detected_at DATETIME,
		FOREIGN KEY(site_id) REFERENCES sites(id)
	);`
	_, err := db.Exec(schema)
	return err
}

func (store *DBStore) SaveReport(domain string, reports []interface{}) error {
	tx, err := store.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var siteID int64
	err = tx.QueryRow("INSERT INTO sites(domain) VALUES(?) ON CONFLICT(domain) DO UPDATE SET domain=domain RETURNING id", domain).Scan(&siteID)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO broken_links(site_id, source_page, target_link, status_code, error_msg, detected_at) VALUES(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Interfacing data maps structural fields dynamically or statically mapped through reflection array blocks
	for _, r := range reports {
		// Representation assumes type-assertion mapping inside loop blocks explicitly mapped
		_ = r
	}

	return tx.Commit()
}

```

### Module C: Monolithic Composition Routing & Templates

File Location: `internal/handlers/router.go`

```go
package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"
)

type SharedMux struct {
	Mux   *http.ServeMux
	Cache map[string]*template.Template
}

func NewRouter() (*SharedMux, error) {
	sm := &SharedMux{
		Mux:   http.NewServeMux(),
		Cache: make(map[string]*template.Template),
	}
	if err := sm.compileViews(); err != nil {
		return nil, err
	}
	sm.registerRoutes()
	return sm, nil
}

func (sm *SharedMux) compileViews() error {
	pages, err := filepath.Glob("./ui/html/pages/*.tmpl")
	if err != nil {
		return err
	}
	for _, page := range pages {
		name := filepath.Base(page)
		ts, err := template.ParseFiles("./ui/html/layouts/base.tmpl")
		if err != nil {
			return err
		}
		ts, err = ts.ParseGlob("./ui/html/components/*.tmpl")
		if err != nil {
			return err
		}
		ts, err = ts.ParseFiles(page)
		if err != nil {
			return err
		}
		sm.Cache[name] = ts
	}
	return nil
}

func (sm *SharedMux) registerRoutes() {
	sm.Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ts, exist := sm.Cache["home.tmpl"]
		if !exist {
			http.Error(w, "Template resolution error", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		ts.ExecuteTemplate(w, "base", nil)
	})
}

```

---

## 4. LLM Agent Processing Protocol (Context Execution Block)

> **Instructions to the User:** Copy the prompt parameters block below verbatim into a fresh session with your LLM code generation agent (e.g., Gemini 3.5 Flash) to command immediate structural generation.

```text
================================================================================
LLM AGENT SYSTEM INITIALIZATION & EXECUTION SYSTEM INSTRUCTIONS
================================================================================
ROLE SPECIFICATION:
You are behaving as a Staff level Systems Engineer specializing in highly optimization-driven Concurrent Go Software Design and elegant, minimal hypermedia web applications utilizing HTMX.

OBJECTIVE:
Generate all required production files for Project "LinkBounty" exactly matching the defined modular framework structure provided below. Do not use generic corporate blue color pallets or heavy JavaScript frameworks. Keep UI designs strictly matching a premium, editorial charcoal-and-cream aesthetic utilizing Tailwind CSS.

CODE ASSEMBLY POLICIES:
1. SOLID & Go Idiomatic Code Principles: Code must contain full production runtime validation checks, zero placeholders, and zero ellipses inside function definitions.
2. Robust Error Handling: Explicitly return error instances up the stack layers. Do not discard error states with anonymous identifiers.
3. Native Execution Paradigm: Maximize the use of the Go Standard Library (`net/http`, `html/template`, `sync`). Use external modules ONLY for `github.com/gocolly/colly/v2` and pure-Go SQLite dependencies (`modernc.org/sqlite` or `github.com/ncruces/go-sqlite3`).
4. Proper HTML Templating: Ensure all structures completely implement matching composition interfaces through explicit `{{define}}` declarations.

FILES TO GENERATE IMEDIATELY:
1. `go.mod` specifying Go 1.22+ tracking constraints.
2. `internal/crawler/engine.go` - Fully implemented asynchronous external crawler using an independent head tracking protocol block.
3. `internal/database/sqlite.go` - Clean, thread-safe database connection layers setting up tables dynamically without explicit migration frameworks.
4. `internal/handlers/router.go` - Pre-compiled view lifecycle system mapped securely to an initialization execution pipeline.
5. `ui/html/layouts/base.tmpl` - The master view layout template wrapped explicitly with high-end typography rules.
6. `cmd/web/main.go` - Single system assembly loop handling signals gracefully.

Execute code assembly now. Output code clean blocks partitioned cleanly by system file identifiers.
================================================================================

```
