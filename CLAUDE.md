# LinkBounty

Free broken link checker for bloggers. Submit a URL → async crawl → shareable UUID report (7-day TTL).

## Stack

- Go 1.25+ · `github.com/gocolly/colly/v2` · `modernc.org/sqlite` (pure Go, no CGO) · `golang.org/x/image` (server-side OG images)
- HTMX 2 · custom CSS design system ("Swiss Signal" theme, Archivo via Google Fonts) · `html/template` (SSR, no JS framework, no Tailwind)
- Single binary + SQLite file — no Docker, no external services

## Commands

```bash
make build    # produces ./linkbounty
make test     # go test ./...
make run      # build + run on :8080 with UI_DIR=./ui/html
make clean    # remove binary and db files
```

Manual run with env overrides:

```bash
ADDR=:9090 DB_PATH=/data/lb.db UI_DIR=./ui/html ./linkbounty
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ADDR` | `:8080` | Listen address |
| `DB_PATH` | `linkbounty.db` | SQLite file path |
| `UI_DIR` | `./ui/html` | Template root directory |
| `SITE_URL` | `https://linkbounty.io` | Public base URL for `robots.txt` + `sitemap.xml` (no trailing slash) |

## Project Layout

```
cmd/web/main.go              entry point, graceful shutdown
internal/
  crawler/engine.go          Colly crawler + net/http external checker
  database/sqlite.go         WAL SQLite store, schema, cleanup
  handlers/
    router.go                App struct, template cache, routes; wraps mux with securityHeaders+accessLog
    middleware.go            securityHeaders (CSP, HSTS, nosniff, frame-options) + accessLog (no IP)
    home.go                  GET /{$} (exact root); catch-all GET / → handle404
    about.go                 GET /a-propos (indexable content page)
    bot.go                   GET /bot — crawler info page (the UA's +URL target)
    legal.go                 GET /confidentialite — privacy page (RGPD)
    notfound.go              handle404 — branded 404 page, HTTP 404, noindex
    scan.go                  POST /scan — validation, rate-limit, job launch
    report.go                GET /r/:uuid — report rendering + SPA/cap logic
    ogimage.go               GET /r/:uuid/og.png — per-report OG image (Go fonts, x/image)
    status.go                GET /r/:uuid/status — HTMX polling fragment
    registry.go              In-memory JobRegistry (atomic.Pointer state per job)
    ratelimit.go             RateLimiter — per-IP throttle with expiry eviction
    robots.go                GET /robots.txt
    seo.go                   GET /sitemap.xml, siteURL() helper, .webmanifest mime
ui/html/
  layouts/base.tmpl          CSS design system, SEO meta + JSON-LD, HTMX, fonts
  pages/                     home, about, bot, confidentialite, notfound, report, error
  components/                progress, link-row
  static/                    favicon.svg, og.png, app icons, site.webmanifest (served at /static/)
deploy/
  linkbounty.service         systemd unit
  Caddyfile                  HTTPS reverse proxy
  deploy.sh                  one-command VPS deploy
```

## Key Architecture Decisions

**Crawler**: Colly crawls internal pages (same `host:port`, max 100). External links are verified in parallel via `net/http` with a semaphore of 20 workers. HEAD first, GET fallback on 405. Respects `robots.txt`.

**Concurrency**: Global scan semaphore of 10 slots (→ 503 when full). Rate limit: 1 scan per IP per 5 minutes via the `RateLimiter` type (mutex-protected map with expiry eviction). Each scan runs under a 10-minute `context` timeout.

**SSRF defense**: Two layers — a hostname check at submission (`scan.go`) for a fast user-facing rejection, plus a resolved-IP check on every dial (`crawler.safeDialContext`) that blocks private/loopback/link-local addresses, defeating DNS rebinding. Tests inject a permissive dialer via `crawler.SetDialContextFunc`.

**Job lifecycle**: `running` → `done` | `error`. In-memory `JobRegistry` for live progress. At startup, any `running` jobs from a prior crash are marked `error`. At SIGTERM, same cleanup before drain.

**HTMX polling**: `/r/:uuid/status` returns a self-replacing `<div hx-trigger="every 2s">` while running, a redirect-on-load fragment when done, and a plain error fragment on failure. The polling loop stops automatically when the trigger is absent.

**SPA detection**: If `ext_links_found < 5` after crawling ≥ 10 pages, an amber warning is shown. Not an error — just a heads-up that the site may use client-side rendering.

**External link cap**: 500 links checked per job. If more are found, a footer note shows the total count.

## SEO

- **Indexing policy**: landing page (`/`) is indexable; report pages (`/r/{uuid}`) and the error page emit `noindex` via the `robots` template block (ephemeral 7-day content). Reports also self-canonicalize and set their own `og:url`/`og:title`. Missing reports return **HTTP 404**.
- **robots.txt** (`robots.go`): allows `/`, disallows `/r/` and `/scan`, links the sitemap.
- **sitemap.xml** (`seo.go`): lists `/` and `/a-propos` (reports are noindex). Add new indexable pages here.
- **Structured data (JSON-LD)**: `Organization` + `WebSite` site-wide in `base.tmpl`; `SoftwareApplication` + `FAQPage` injected by `home.tmpl` via the `jsonld` block. The visible FAQ section must stay in sync with the `FAQPage` text (Google requires matching visible content).
- **Social**: Open Graph + Twitter `summary_large_image`, `og:locale fr_FR`. Default OG image `/static/og.png` (1200×630). Report pages override `og:image` to a **dynamic** per-report PNG at `/r/{uuid}/og.png` (drawn server-side in `ogimage.go` with the embedded Go fonts via `golang.org/x/image`; shows the domain + broken-link count). The `og:image` block is defined once in `base.tmpl` and re-invoked for `twitter:image` with `{{template "og:image" .}}` (a second `{{block}}` would be a duplicate-definition error).
- **Routing**: home is `GET /{$}` (exact match); any other unmatched path hits the catch-all `GET /` → `handle404` (branded 404 page, HTTP 404, noindex). Unknown reports return 404. `/bot` documents the crawler (the `+URL` in the bot's User-Agent now resolves).
- **Security & observability**: `securityHeaders` middleware sets CSP, HSTS, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy` (the CSP allows `'unsafe-inline'` for the SSR inline `<style>`/handlers + the htmx and Google Fonts origins). `accessLog` middleware logs `method/path/status/duration` with **no IP** (consistent with the privacy page). Caddy handles compression + TLS and defers security headers to the app.
- **Icons/PWA**: `favicon.svg`, `apple-touch-icon.png`, `icon-192/512.png`, `site.webmanifest` under `ui/html/static/`. `/favicon.ico` 301-redirects to the SVG.
- **Domain**: absolute URLs in templates are hardcoded `https://linkbounty.io`; server-generated robots/sitemap use `SITE_URL`. Change both when deploying under a new domain.
- Per-page overrides use template blocks: `robots`, `canonical`, `og:url`, `og:title`, `og:description`, `twitter:*`, `jsonld`.

## Database Schema

```sql
jobs(id TEXT PK, domain, status, error_msg, pages_crawled, broken_count,
     ext_links_found, created_at)

broken_links(id INT PK, job_id REFERENCES jobs ON DELETE CASCADE,
             source_page, target_link, status_code, error_msg, link_type)
-- link_type: 'broken' | 'redirect' | 'unverifiable' (e.g. LinkedIn 999 anti-bot)
```

WAL mode, `PRAGMA foreign_keys=ON`, `PRAGMA wal_autocheckpoint=100`, `PRAGMA busy_timeout=5000`,
single open connection. Schema versioned via `PRAGMA user_version` (append-only `migrations` list).
7-day TTL, hourly cleanup goroutine.

## Testing

```bash
make test                      # all packages
go test -v ./internal/crawler/ # with output
```

CGO is not required (pure-Go SQLite). Race detector requires `CGO_ENABLED=1` and a C compiler.

## Deploying

```bash
./deploy/deploy.sh user@your-vps-ip
```

Prerequisites on the VPS: Caddy installed, port 80/443 open.
Copy `deploy/Caddyfile` to `/etc/caddy/Caddyfile` and edit the domain before deploying.

## Phase 2 Gateway

Ship when: at least one person shares their report link within 2 weeks of launch.
If yes → scheduled monitoring, accounts, subscriptions.
If no → revisit distribution before building more.
