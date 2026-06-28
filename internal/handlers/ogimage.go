package handlers

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Per-report Open Graph image (1200x630) drawn server-side so social shares of
// a report show the domain and broken-link count. Uses the embedded Go fonts
// (no external font file). Report pages are noindex; this is for sharing only.

const (
	ogW = 1200
	ogH = 630
)

var (
	ogInk    = color.RGBA{0x0a, 0x0a, 0x0a, 0xff}
	ogSignal = color.RGBA{0xff, 0x41, 0x24, 0xff}
	ogGray   = color.RGBA{0x6b, 0x6b, 0x6b, 0xff}
	ogGreen  = color.RGBA{0x0f, 0x7a, 0x37, 0xff}
	ogWhite  = color.RGBA{0xff, 0xff, 0xff, 0xff}

	ogBold    = mustFont(gobold.TTF)
	ogRegular = mustFont(goregular.TTF)
)

func mustFont(b []byte) *opentype.Font {
	f, err := opentype.Parse(b)
	if err != nil {
		panic(err)
	}
	return f
}

func ogFace(f *opentype.Font, px float64) font.Face {
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	return face
}

func (a *App) handleReportOG(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("uuid")
	job, err := a.DB.GetJob(id)
	if err != nil || job == nil {
		// Unknown/expired report: fall back to the static brand image.
		http.Redirect(w, r, "/static/og.png", http.StatusFound)
		return
	}

	img := image.NewRGBA(image.Rect(0, 0, ogW, ogH))
	draw.Draw(img, img.Bounds(), image.NewUniform(ogWhite), image.Point{}, draw.Src)

	// Border frame (2px, inset 24px).
	fillRect(img, 24, 24, ogW-24, 26, ogInk)
	fillRect(img, 24, ogH-26, ogW-24, ogH-24, ogInk)
	fillRect(img, 24, 24, 26, ogH-24, ogInk)
	fillRect(img, ogW-26, 24, ogW-24, ogH-24, ogInk)

	// Wordmark + orange dot.
	wm := ogFace(ogBold, 30)
	defer wm.Close()
	end := drawText(img, wm, ogInk, 64, 96, "LINKBOUNTY")
	fillRect(img, end+10, 78, end+24, 92, ogSignal)

	// Top-right kicker.
	kf := ogFace(ogRegular, 17)
	defer kf.Close()
	drawTextRight(img, kf, ogGray, ogW-64, 94, "RAPPORT D'INSPECTION")

	// Domain (uppercase), shrink to fit the content width.
	domain := strings.ToUpper(job.Domain)
	maxW := ogW - 128
	var df font.Face
	for _, px := range []float64{92, 76, 60, 48} {
		df = ogFace(ogBold, px)
		if textWidth(df, domain) <= maxW {
			break
		}
		df.Close()
	}
	defer df.Close()
	drawText(img, df, ogInk, 64, 320, domain)

	// Orange accent bar under the domain.
	fillRect(img, 64, 352, 184, 362, ogSignal)

	// Count line.
	cf := ogFace(ogBold, 34)
	defer cf.Close()
	gf := ogFace(ogRegular, 28)
	defer gf.Close()
	brokenCol := ogSignal
	brokenTxt := plural(job.BrokenCount, "lien brisé", "liens brisés")
	if job.BrokenCount == 0 {
		brokenCol = ogGreen
		brokenTxt = "Aucun lien brisé"
	}
	x := drawText(img, cf, brokenCol, 64, 440, brokenTxt)
	drawText(img, gf, ogGray, x+18, 440, "·  "+plural(job.PagesCrawled, "page analysée", "pages analysées"))

	// Footer hint.
	ff := ogFace(ogRegular, 22)
	defer ff.Close()
	drawText(img, ff, ogGray, 64, ogH-72, "linkbounty.io")

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_ = png.Encode(w, img)
}

// plural renders "<n> <singular|plural>" with French agreement (n<=1 → singular).
func plural(n int, one, many string) string {
	if n <= 1 {
		return itoa(n) + " " + one
	}
	return itoa(n) + " " + many
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	draw.Draw(img, image.Rect(x0, y0, x1, y1), image.NewUniform(c), image.Point{}, draw.Src)
}

// drawText draws s with its baseline at (x, y) and returns the x after the text.
func drawText(img *image.RGBA, face font.Face, c color.Color, x, y int, s string) int {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
	return d.Dot.X.Ceil()
}

func drawTextRight(img *image.RGBA, face font.Face, c color.Color, xRight, y int, s string) {
	drawText(img, face, c, xRight-textWidth(face, s), y, s)
}

func textWidth(face font.Face, s string) int {
	d := &font.Drawer{Face: face}
	return d.MeasureString(s).Ceil()
}
