# LinkBounty

**Free broken link checker for bloggers and content creators.**

Submit a URL → LinkBounty crawls your site, checks every outbound link, and gives you a shareable report in under a minute. No account, no plugin, no tracking.

---

## Why

Broken links silently cost you SEO ranking and reader trust. Tools that find them either cost $99/month (Ahrefs, Screaming Frog) or require installing a WordPress plugin that slows your site. LinkBounty is a free, frictionless alternative for blogs with 50–500 articles.

---

## Features

- **Zero friction** — paste URL, get report. No signup, no email.
- **Shareable reports** — every scan gets a unique link valid for 7 days.
- **Fast** — up to 100 pages crawled concurrently, 20 external link checks in parallel.
- **Honest about limits** — warns when a site uses client-side rendering (SPA) or when more than 500 external links were found.
- **Redirect tracking** — 3xx redirects shown separately, not counted as broken.
- **Respects robots.txt** — LinkBountyBot follows `Disallow` rules of target sites.

---

## Quick Start (development)

**Requirements:** Go 1.25+

```bash
git clone https://github.com/yourusername/linkbounty
cd linkbounty
make run
# → LinkBounty listening on :8080
```

Open `http://localhost:8080`, paste any URL, and scan.

---

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `ADDR` | `:8080` | HTTP listen address |
| `DB_PATH` | `linkbounty.db` | SQLite database file path |
| `UI_DIR` | `./ui/html` | HTML template directory |

Example with custom values:

```bash
ADDR=:9000 DB_PATH=/var/lib/linkbounty/data.db ./linkbounty
```

---

## Deployment

LinkBounty ships as a single static binary. The recommended setup is a Hetzner CX11 (€4/mo) with Caddy as a TLS reverse proxy.

### 1. Build for Linux

```bash
GOOS=linux GOARCH=amd64 make build
```

### 2. Deploy in one command

```bash
./deploy/deploy.sh user@your-vps-ip
```

This script:
- Cross-compiles the binary for Linux/amd64
- `rsync`s the binary and UI templates to `/opt/linkbounty`
- Installs and starts a `systemd` service as the `linkbounty` user

### 3. Configure Caddy (TLS + reverse proxy)

Edit `deploy/Caddyfile` with your domain, then copy it to the VPS:

```bash
scp deploy/Caddyfile user@your-vps:/etc/caddy/Caddyfile
ssh user@your-vps sudo systemctl reload caddy
```

Caddy handles HTTPS automatically via Let's Encrypt.

### Manual systemd setup

```bash
# On the VPS
sudo useradd --system --no-create-home linkbounty
sudo mkdir -p /opt/linkbounty
sudo chown linkbounty: /opt/linkbounty
sudo cp linkbounty /opt/linkbounty/
sudo cp -r ui/ /opt/linkbounty/
sudo cp deploy/linkbounty.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now linkbounty
```

---

## Architecture

```
POST /scan
  │ validate URL (scheme, host, blocklist)
  │ rate-limit: 1 scan/5min per IP
  │ global semaphore: max 10 concurrent scans
  │ create job in SQLite (status=running)
  │ launch goroutine
  └─→ 302 /r/:uuid

goroutine
  │ Colly crawls internal pages (max 100, respects robots.txt)
  │ for each outbound link → goroutine pool (max 20)
  │   HEAD request → GET fallback on 405
  │   classify: 2xx=ok, 3xx=redirect, 4xx/5xx/err=broken
  │ update SQLite (status=done, counts, links)
  └─→ JobRegistry updated (atomic, polled by /status)

GET /r/:uuid/status  (HTMX poll every 2s)
  │ read JobRegistry (in-memory, fast)
  │ fallback to SQLite if job not in registry (server restart)
  └─→ HTML fragment: spinner | redirect-to-report | error

GET /r/:uuid
  └─→ full report: grouped by source page, redirects section,
       SPA warning, truncation note, share button
```

### Tech stack

| Layer | Choice | Why |
|-------|--------|-----|
| Language | Go 1.25 | Single binary, excellent concurrency, easy deploy |
| Crawler | Colly v2 | Event-driven, robots.txt, rate limiting built in |
| External checks | `net/http` + goroutines | Colly not needed for single requests |
| Database | SQLite (`modernc.org/sqlite`) | Pure Go, zero config, WAL for concurrent reads |
| Frontend | HTMX + Tailwind CDN v4 | No build step, no JS framework, SSR |
| Proxy | Caddy | Auto-HTTPS, simple config |
| Process | systemd | No Docker overhead on a €4 VPS |

---

## Development

### Running tests

```bash
make test
# or with verbose output:
go test -v ./...
```

Tests use real SQLite (`:memory:`) and real `httptest.Server` instances — no mocks.

### Project structure

```
cmd/web/main.go          server entry point
internal/
  crawler/               Colly crawler + external link checker
  database/              SQLite store (schema, queries, cleanup)
  handlers/              HTTP handlers (scan, report, status, robots)
ui/html/
  layouts/               base template (CSS tokens, SEO, HTMX)
  pages/                 home, report, error
  components/            progress spinner, link row
deploy/                  systemd service, Caddyfile, deploy script
```

### Adding a new handler

1. Create `internal/handlers/myhandler.go` with a method on `*App`
2. Register the route in `(*App).Routes()` in `router.go`
3. Add a template in `ui/html/pages/` if needed

### Database changes

Edit `migrate()` in `internal/database/sqlite.go`. Since the schema uses `CREATE TABLE IF NOT EXISTS`, adding columns requires an `ALTER TABLE` statement for existing databases. New installs pick up the full schema automatically.

---

## Limits (Phase 1)

| Limit | Value | Reason |
|-------|-------|--------|
| Pages crawled per scan | 100 | Keeps scans under 60s on a CX11 |
| External links checked | 500 | Prevents runaway scans on large sites |
| Concurrent scans | 10 | ~200 outbound connections total |
| Rate limit | 1 scan / 5 min / IP | Basic abuse prevention |
| Report TTL | 7 days | Long enough to share; short enough to manage disk |

JavaScript-rendered sites (React, Vue, SPA) are not supported — Colly does not execute JavaScript. A warning is displayed when fewer than 5 external links are found after crawling 10+ pages.

---

## Roadmap

Phase 1 (this repo) is intentionally minimal. The gate to Phase 2 is:

> **Has anyone shared their report link within 2 weeks of launch?**

If yes → Phase 2: scheduled monitoring, email alerts, user accounts, subscriptions.

---

## License

MIT
