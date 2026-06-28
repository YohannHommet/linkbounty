# LinkBounty — TODO

Status snapshot of the project: what's done, what's left before launch, and what's deferred.
Last updated during the pre-launch finishing pass.

---

## ✅ Done

### Code quality & architecture
- [x] Decoupled `crawler` from `database` (`crawler.Link` type; mapping in handlers)
- [x] `crawler.Config` + `DefaultConfig()`; deduplicated HEAD→GET retry (`headWithGETFallback`)
- [x] Extracted `runJob` (service) and `buildReportData` (pure mapper, tested)
- [x] `RateLimiter` type (per-IP, expiry eviction); `JobRegistry` on `atomic.Pointer`
- [x] Shared `JobStatus` constants in `database`
- [x] SQLite hardening: `busy_timeout`, single open conn, versioned migrations (`user_version`)
- [x] Security: XSS escaping on error messages, SSRF dial-time IP check (DNS-rebinding safe), rate-limit not spoofable via XFF
- [x] Context propagation + 10-min scan timeout; TOCTOU fixes
- [x] Tests: RateLimiter, buildReportData, crawler cap, status fragments
- [x] `go build` / `vet` / `gofmt` / `go test` all green

### Design — "Swiss Signal" theme
- [x] Full redesign in custom CSS (Archivo), **Tailwind removed** (no prod warning)
- [x] All screens: home, report (running/done/clean/error), about, 404, bot, privacy
- [x] Responsive (mobile verified), entrance/states polished
- [x] CSS-variable contract preserved for server-rendered HTMX fragments

### SEO
- [x] `robots.txt` (allow `/`, disallow `/r/` + `/scan`, sitemap link)
- [x] `sitemap.xml` (`/`, `/a-propos`, `/confidentialite`)
- [x] Report + error + bot pages `noindex`; reports self-canonical + dynamic OG
- [x] JSON-LD: Organization + WebSite (site-wide), SoftwareApplication + FAQPage (home), AboutPage
- [x] Open Graph + Twitter `summary_large_image`, `og:locale fr_FR`
- [x] Static OG image (1200×630) + **dynamic per-report OG** (`/r/{uuid}/og.png`)
- [x] favicon.svg (dark-mode aware) + apple-touch-icon + icons + `site.webmanifest`
- [x] Visible FAQ section (matches FAQPage schema) + `/a-propos` content page
- [x] `theme-color` = brand orange

### Finishing touches (A+B+C+D)
- [x] **A1** soft-404 fixed: `GET /{$}` + branded 404 page (HTTP 404, noindex)
- [x] **A2** `/bot` page (the crawler User-Agent `+URL` now resolves)
- [x] **B4** security headers middleware (CSP, HSTS, nosniff, frame-options, referrer-policy)
- [x] **B5/D13** access-log middleware (method/path/status/duration, **no IP**)
- [x] **C7** contrast bumped (grays + small orange text), `:focus-visible` outlines
- [x] **C8** route/SEO tests (`seo_test.go`)
- [x] **C9** `/confidentialite` privacy page (RGPD, indexable)
- [x] **D10** favicon dark-mode
- [x] **D11** dynamic per-report OG image (Go fonts via `golang.org/x/image`)
- [x] **D12** README rewrite, MIT LICENSE, moved planning doc to `docs/`

---

## 🚧 Blockers before launch (need a decision / real value)

- [x] **GitHub repo URL** — done: `https://github.com/YohannHommet/linkbounty` (public), links updated in `base.tmpl`, code pushed.
- [ ] **Production domain** — `https://linkbounty.io` is hardcoded in templates (canonical/OG) and is the default `SITE_URL`. If it changes, update: `base.tmpl`, the `siteURL()` default in `seo.go`, `deploy/Caddyfile` (PLACEHOLDER), `deploy/linkbounty.service` (PLACEHOLDER)
- [ ] **Register the domain + DNS** (A/AAAA to the VPS)
- [ ] **License** — MIT assumed; confirm or replace `LICENSE`

---

## 🚀 Launch checklist (deploy)

- [ ] Provision VPS: Caddy installed, ports 80/443 open
- [ ] Edit `deploy/Caddyfile` domain + `deploy/linkbounty.service` (replace PLACEHOLDER)
- [ ] Set env on the box: `SITE_URL`, `DB_PATH`, `UI_DIR`, `ADDR`
- [ ] Run `./deploy/deploy.sh user@vps-ip`
- [ ] Verify on the live domain: HTTPS, security headers, `/robots.txt`, `/sitemap.xml`, favicon, OG preview
- [ ] Submit sitemap to Google Search Console + Bing Webmaster
- [ ] Smoke test: real scan → report → share link → `/r/{uuid}/og.png` → 404 page

---

## 🧪 Hardening (recommended, non-blocking)

- [x] **Race detector** — runs in CI (`go test -race`) on the Ubuntu runner; first run green, no data races.
- [x] **CI** — `.github/workflows/ci.yml` runs gofmt + vet + build + test -race on push/PR. First run: ✅ green.
- [ ] **Test gaps** — no test for the OG image handler (`ogimage.go`) or rendering of new pages (bot/privacy/404)
- [ ] **Mobile** — visually verify `/bot`, `/confidentialite`, 404 at mobile width (likely fine, same responsive pattern, not screenshotted)
- [ ] **A11y** — primary button is white-on-#ff4124 (~3.4:1), below WCAG AA for 12px text; either accept as a brand tradeoff or darken the button bg
- [x] **Commit & push** — work committed and pushed to `github.com/YohannHommet/linkbounty` (branch `develop`)

---

## 📈 Measurement (Phase 2 gateway)

The Phase 2 gate (`CLAUDE.md`): *ship more only if ≥1 person shares their report link within 2 weeks of launch.*
- [ ] Decide how to measure it. Today: count `GET /r/{uuid}` lines in the access log (no IP). For a real dashboard, add privacy-friendly analytics (Plausible / Umami) — one script tag.

---

## 🎨 Nice-to-have / deferred

- [ ] Real multi-resolution `favicon.ico` (currently `/favicon.ico` 301-redirects to the SVG)
- [ ] Embed Archivo TTF in the OG image (currently uses the built-in Go font; cosmetic)
- [ ] Richer dynamic OG (status-code badges, not just the count)
- [ ] `font-src 'self'` in CSP if self-hosting fonts later

---

## 🔮 Phase 2 (post-launch, gated — do NOT build yet)

Scheduled monitoring, accounts, subscriptions. Only after the gateway signal fires.
If it doesn't fire → revisit distribution before building more.
