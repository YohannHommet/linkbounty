package handlers

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"strings"
)

func init() {
	// Go's mime table doesn't know .webmanifest; register it so the static
	// file server serves the PWA manifest with the correct content type.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// siteURL returns the public base URL (no trailing slash), used for absolute
// references in robots.txt and the sitemap. Override with the SITE_URL env var
// when deploying under a different domain.
func siteURL() string {
	if v := os.Getenv("SITE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://linkbounty.io"
}

// handleSitemap serves a minimal sitemap. Only the public landing page is
// indexable — report pages (/r/{uuid}) are ephemeral and marked noindex.
func (a *App) handleSitemap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	base := siteURL()
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>%s/</loc>
    <changefreq>weekly</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>%s/a-propos</loc>
    <changefreq>monthly</changefreq>
    <priority>0.7</priority>
  </url>
  <url>
    <loc>%s/confidentialite</loc>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
</urlset>
`, base, base, base)
}
