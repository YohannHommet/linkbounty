# LinkBounty

**Free broken-link checker for bloggers and content creators.**

Submit a URL → LinkBounty crawls your site, checks every outbound link, and gives you a shareable report. No account, no plugin, no tracking.

---

## Features

- **Zero friction** — paste a URL, get a report; no signup, no email required
- **Crawls up to 100 pages** per scan, respecting `robots.txt`
- **External link verification** — HEAD request with automatic GET fallback on 405
- **Classifies results** — 404/410/500 broken, 3xx redirects, unverifiable (anti-bot), and clean links reported separately
- **SSRF-protected** — hostname blocklist at submission + per-dial resolved-IP check (defeats DNS rebinding)
- **Shareable reports** — every scan gets a unique link valid for 7 days
- **SEO-aware** — JSON-LD structured data, Open Graph, `robots.txt`, `sitemap.xml`
- **SPA detection** — warns when a site appears to use client-side rendering
- **No CGO required** — pure-Go SQLite, single binary, deploys anywhere

---

## Tech Stack

| Layer | Choice |
|-------|--------|
| Language | Go 1.25+ |
| Crawler | `github.com/gocolly/colly/v2` |
| External checks | `net/http` + goroutine pool (20 workers) |
| Frontend | `html/template` (SSR) + HTMX 2 |
| CSS | Custom "Swiss Signal" design system (no Tailwind, no build step) |
| Database | `modernc.org/sqlite` — pure Go, WAL mode, no CGO |
| Deployment | Single binary + Caddy + systemd |

---

## Quick Start

**Requirements:** Go 1.25+

```bash
git clone https://github.com/yourusername/linkbounty
cd linkbounty
make run
# → LinkBounty listening on :8080
```

Open `http://localhost:8080`, paste any URL, and scan.

### Make targets

```bash
make build   # produces ./linkbounty
make test    # go test ./...
make run     # build + run on :8080 with UI_DIR=./ui/html
make clean   # remove binary and db files
```

---

## Configuration

All configuration is via environment variables. No config file needed.

| Variable | Default | Description |
|----------|---------|-------------|
| `ADDR` | `:8080` | HTTP listen address |
| `DB_PATH` | `linkbounty.db` | SQLite database file path |
| `UI_DIR` | `./ui/html` | HTML template directory |
| `SITE_URL` | `https://linkbounty.io` | Public base URL used in `robots.txt` and `sitemap.xml` (no trailing slash) |

Example with custom values:

```bash
ADDR=:9090 DB_PATH=/var/lib/linkbounty/data.db UI_DIR=./ui/html SITE_URL=https://example.com ./linkbounty
```

---

## Project Layout

```
cmd/web/main.go              Entry point, graceful shutdown (SIGTERM)
internal/
  crawler/engine.go          Colly crawler + net/http external checker
  database/sqlite.go         WAL SQLite store, schema, migrations, 7-day cleanup
  handlers/
    router.go                App struct, template cache, route registration
    home.go                  GET /
    about.go                 GET /a-propos
    scan.go                  POST /scan — validation, rate-limit, job launch
    report.go                GET /r/{uuid} — report page
    status.go                GET /r/{uuid}/status — HTMX polling fragment
    registry.go              In-memory JobRegistry (atomic state per job)
    ratelimit.go             Per-IP throttle (1 scan / 5 min)
    robots.go                GET /robots.txt
    seo.go                   GET /sitemap.xml
ui/html/
  layouts/base.tmpl          CSS design system, SEO meta + JSON-LD, HTMX, fonts
  pages/                     home, about, report, error, privacy, bot
  components/                progress spinner, link row
  static/                    favicon.svg, og.png, app icons, site.webmanifest
deploy/
  linkbounty.service         systemd unit
  Caddyfile                  HTTPS reverse proxy config
  deploy.sh                  One-command VPS deploy script
docs/
  architecture-blueprint.md  Original planning document
```

---

## Public Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Landing page |
| GET | `/a-propos` | About page (indexable) |
| GET | `/bot` | Bot / crawler info page |
| GET | `/confidentialite` | Privacy policy |
| POST | `/scan` | Submit a URL for scanning |
| GET | `/r/{uuid}` | Scan report (noindex, 7-day TTL) |
| GET | `/r/{uuid}/status` | HTMX polling fragment (in-progress state) |
| GET | `/robots.txt` | Crawler policy |
| GET | `/sitemap.xml` | XML sitemap |
| GET | `/static/*` | Static assets (favicon, icons, OG image) |
| GET | `/favicon.ico` | 301 → `/static/favicon.svg` |

---

## Deployment

LinkBounty ships as a single static binary (no Docker required). The recommended setup is a VPS with Caddy as a TLS reverse proxy.

### One-command deploy

```bash
./deploy/deploy.sh user@your-vps-ip
```

This script cross-compiles the binary for Linux/amd64, rsyncs the binary and UI templates to `/opt/linkbounty`, and installs a `systemd` service.

### Caddy (HTTPS)

Edit `deploy/Caddyfile` with your domain, then:

```bash
scp deploy/Caddyfile user@your-vps:/etc/caddy/Caddyfile
ssh user@your-vps sudo systemctl reload caddy
```

Caddy handles HTTPS automatically via Let's Encrypt.

### Environment for production

Set `SITE_URL` to your public domain so that `robots.txt` and `sitemap.xml` contain correct absolute URLs:

```
SITE_URL=https://yourdomain.com
```

### CGO note

CGO is **not required**. The pure-Go SQLite driver (`modernc.org/sqlite`) is used throughout. Cross-compilation works without a C toolchain. The race detector (`go test -race`) does require `CGO_ENABLED=1` and a C compiler.

---

## Operational Limits

| Limit | Value |
|-------|-------|
| Pages crawled per scan | 100 |
| External links checked | 500 |
| Concurrent scans (global) | 10 |
| Rate limit | 1 scan / 5 min / IP |
| Report TTL | 7 days |

JavaScript-rendered sites (React, Vue, SPA) are not supported — Colly does not execute JavaScript. A warning is shown when fewer than 5 external links are found after crawling 10+ pages.

---

## License

MIT — see [LICENSE](LICENSE).
