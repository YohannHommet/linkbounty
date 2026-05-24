# LinkBounty

Free broken link checker for bloggers. Submit a URL → async crawl → shareable UUID report (7-day TTL).

## Stack

- Go 1.25+ · `github.com/gocolly/colly/v2` · `modernc.org/sqlite` (pure Go, no CGO)
- HTMX 2 · Tailwind CDN v4 · `html/template` (SSR, no JS framework)
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

## Project Layout

```
cmd/web/main.go              entry point, graceful shutdown
internal/
  crawler/engine.go          Colly crawler + net/http external checker
  database/sqlite.go         WAL SQLite store, schema, cleanup
  handlers/
    router.go                App struct, template cache, route registration
    home.go                  GET /
    scan.go                  POST /scan — validation, rate-limit, job launch
    report.go                GET /r/:uuid — report rendering + SPA/cap logic
    status.go                GET /r/:uuid/status — HTMX polling fragment
    registry.go              In-memory JobRegistry (atomic state per job)
    robots.go                GET /robots.txt
ui/html/
  layouts/base.tmpl          CSS tokens, SEO meta, HTMX, fonts
  pages/                     home, report, error
  components/                progress, link-row
deploy/
  linkbounty.service         systemd unit
  Caddyfile                  HTTPS reverse proxy
  deploy.sh                  one-command VPS deploy
```

## Key Architecture Decisions

**Crawler**: Colly crawls internal pages (same `host:port`, max 100). External links are verified in parallel via `net/http` with a semaphore of 20 workers. HEAD first, GET fallback on 405. Respects `robots.txt`.

**Concurrency**: Global scan semaphore of 10 slots (→ 503 when full). Rate limit: 1 scan per IP per 5 minutes, `sync.Mutex` protected.

**Job lifecycle**: `running` → `done` | `error`. In-memory `JobRegistry` for live progress. At startup, any `running` jobs from a prior crash are marked `error`. At SIGTERM, same cleanup before drain.

**HTMX polling**: `/r/:uuid/status` returns a self-replacing `<div hx-trigger="every 2s">` while running, a redirect-on-load fragment when done, and a plain error fragment on failure. The polling loop stops automatically when the trigger is absent.

**SPA detection**: If `ext_links_found < 5` after crawling ≥ 10 pages, an amber warning is shown. Not an error — just a heads-up that the site may use client-side rendering.

**External link cap**: 500 links checked per job. If more are found, a footer note shows the total count.

## Database Schema

```sql
jobs(id TEXT PK, domain, status, error_msg, pages_crawled, broken_count,
     ext_links_found, created_at)

broken_links(id INT PK, job_id REFERENCES jobs ON DELETE CASCADE,
             source_page, target_link, status_code, error_msg, link_type)
-- link_type: 'broken' | 'redirect'
```

WAL mode, `PRAGMA foreign_keys=ON`, `PRAGMA wal_autocheckpoint=100`. 7-day TTL, hourly cleanup goroutine.

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
