package ui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/mattn/go-sixel"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var imgHTTPClient = &http.Client{Timeout: 10 * time.Second}

func downloadImage(url string) (image.Image, error) {
	resp, err := imgHTTPClient.Get(url) //nolint:noctx
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	img, _, err := image.Decode(resp.Body)
	return img, err
}

// imageDims computes terminal cols/rows for an image while preserving its aspect ratio.
// Terminal cells are ~2:1 (height:width), so displayed aspect ratio = cols / (rows * 2).
func imageDims(src image.Image, maxCols, maxRows int) (cols, rows int) {
	b := src.Bounds()
	imgW := b.Max.X - b.Min.X
	imgH := b.Max.Y - b.Min.Y
	if imgW == 0 || imgH == 0 {
		return maxCols, maxRows
	}

	cols = maxCols
	rows = imgH * cols / (imgW * 2)
	if rows < 1 {
		rows = 1
	}

	if rows > maxRows {
		rows = maxRows
		cols = imgW * rows * 2 / imgH
		if cols < 1 {
			cols = 1
		}
		if cols > maxCols {
			cols = maxCols
		}
	}
	return cols, rows
}

// supportsSixel reports whether Sixel output is enabled.
//
// Sixel is opt-in: guessing from TERM_PROGRAM/TERM is unreliable because those
// variables survive terminal multiplexers (tmux, zellij, herdr, ...) that do not
// forward Sixel DCS sequences, in which case the image is silently swallowed and
// nothing is drawn at all. The half-block renderer is plain text plus 24-bit
// colour, so it works everywhere; set BSKY_SIXEL=1 to opt into Sixel.
func supportsSixel() bool {
	return os.Getenv("BSKY_SIXEL") == "1"
}

const (
	// Chunk size mandated by the Kitty graphics protocol for escape-code transmission.
	kittyChunkBytes = 4096
	// Fixed image and placement IDs. A zero placement ID is an "internal" one, which
	// lets placements stack: every repaint would add another copy of the same image.
	// Non-zero IDs mean one image and one placement that later repaints replace.
	kittyImageID     = 8151
	kittyPlacementID = 1
)

// supportsKitty reports whether the Kitty graphics protocol can be used.
//
// Unlike Sixel this can be auto-detected. Kitty graphics survive herdr, which parses
// the APC sequence into its own terminal state and re-emits it to the outer terminal,
// so the multiplexer concern that keeps Sixel opt-in does not apply to it. tmux and
// zellij do not forward the sequence, so they stay on the half-block renderer.
// BSKY_KITTY=1 forces it on, BSKY_KITTY=0 forces it off.
func supportsKitty() bool {
	if v := os.Getenv("BSKY_KITTY"); v != "" {
		return v == "1"
	}
	if os.Getenv("TMUX") != "" || os.Getenv("ZELLIJ") != "" {
		return false
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	switch os.Getenv("TERM") {
	case "xterm-kitty", "xterm-ghostty":
		return true
	}
	switch strings.ToLower(os.Getenv("TERM_PROGRAM")) {
	case "wezterm", "ghostty":
		return true
	}
	return false
}

// renderImageForView renders src into a string suitable for embedding in BubbleTea's View().
//
// availableRows is the exact number of terminal rows reserved for the image in the layout.
// The returned string always contributes exactly (availableRows-1) newline characters so
// that BubbleTea's line counter matches the visual space the image occupies.
//
// Sixel strategy (when supported):
//
//	The string is built as:
//	  "\n" × (availableRows-1)          ← placeholder: BubbleTea counts these lines
//	  "\033[<availableRows-1>A"          ← cursor-up: return to the start of the image area
//	  <Sixel DCS>                        ← pixels rendered; cursor ends at bottom of image ✓
//
//	BubbleTea renders the empty placeholder lines first, then the Sixel line overwrites
//	them.  On subsequent diff-renders, unchanged lines are skipped, so the Sixel persists.
//
// Kitty strategy (when supported):
//
//	The same placeholder trick, with a Kitty APC sequence in place of the Sixel DCS.
//	See renderImageKittyView.
//
// Block-char fallback:
//
//	pixterm half-block (▀) rendering, padded to exactly availableRows rows.
func renderImageForView(src image.Image, maxCols, availableRows int) string {
	if availableRows < 1 {
		return ""
	}
	// Sixel first: it is an explicit opt-in, so it wins over auto-detected Kitty.
	if supportsSixel() {
		if s := renderImageSixelView(src, maxCols, availableRows); s != "" {
			return s
		}
	}
	if supportsKitty() {
		if s := renderImageKittyView(src, maxCols, availableRows); s != "" {
			return s
		}
	}
	return renderImageBlockView(src, maxCols, availableRows)
}

// renderImageSixelView builds the Sixel image string with cursor-up placeholder trick.
// The Sixel image is (availableRows-1) terminal rows tall; one row is "spent" on the
// cursor-up manoeuvre so BubbleTea's line count stays exact.
func renderImageSixelView(src image.Image, maxCols, availableRows int) string {
	sixelRows := availableRows - 1
	if sixelRows < 1 {
		return ""
	}
	cols, rows := imageDims(src, maxCols, sixelRows)
	pixW := cols * 8
	pixH := rows * 16
	dst := image.NewRGBA(image.Rect(0, 0, pixW, pixH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	enc := sixel.NewEncoder(&buf)
	if err := enc.Encode(dst); err != nil {
		return ""
	}

	// "\n" × sixelRows → placeholder lines (BubbleTea counts them)
	// cursor-up rows     → back to image area start
	// sixelData          → pixels drawn; cursor ends rows below start ✓
	placeholder := strings.Repeat("\n", sixelRows)
	cursorUp := fmt.Sprintf("\033[%dA", rows)
	return placeholder + cursorUp + buf.String()
}

// renderImageKittyView builds the Kitty graphics image string, using the same
// placeholder trick as the Sixel path so BubbleTea's line count stays exact:
//
//	"\n" × (availableRows-1)   ← placeholder lines BubbleTea counts
//	cursor-up rows             ← back to the start of the image area
//	<Kitty APC chunks>         ← C=1 keeps the cursor where it is
//	cursor-down rows           ← leave the cursor where the placeholder left it
//
// q=2 suppresses the terminal's success and error replies. Without it they arrive on
// stdin and BubbleTea reads them as key presses.
func renderImageKittyView(src image.Image, maxCols, availableRows int) string {
	kittyRows := availableRows - 1
	if kittyRows < 1 {
		return ""
	}
	cols, rows := imageDims(src, maxCols, kittyRows)

	// Downscale before encoding: source images run to several megapixels and this
	// re-encodes on every repaint. The terminal scales the result into the c×r cell box.
	dst := image.NewRGBA(image.Rect(0, 0, cols*8, rows*16))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return ""
	}
	payload := base64.StdEncoding.EncodeToString(buf.Bytes())

	var sb strings.Builder
	sb.WriteString(strings.Repeat("\n", kittyRows))
	fmt.Fprintf(&sb, "\033[%dA", rows)
	for first := true; len(payload) > 0; first = false {
		chunk := payload
		if len(chunk) > kittyChunkBytes {
			chunk = chunk[:kittyChunkBytes]
		}
		payload = payload[len(chunk):]
		more := 0
		if len(payload) > 0 {
			more = 1
		}
		if first {
			// a=T transmits and displays in one command, f=100 is PNG.
			fmt.Fprintf(&sb, "\033_Ga=T,f=100,i=%d,p=%d,c=%d,r=%d,C=1,q=2,m=%d;%s\033\\",
				kittyImageID, kittyPlacementID, cols, rows, more, chunk)
		} else {
			// Continuation chunks carry only m; the terminal remembers the rest.
			fmt.Fprintf(&sb, "\033_Gm=%d;%s\033\\", more, chunk)
		}
	}
	fmt.Fprintf(&sb, "\033[%dB", rows)
	return sb.String()
}

// renderImageBlockView renders src as half-block characters (▀) with ANSI 24-bit colour,
// padded/trimmed to exactly availableRows terminal rows ((availableRows-1) newlines).
func renderImageBlockView(src image.Image, maxCols, availableRows int) string {
	bg := color.RGBA{A: 255}
	// NoDithering uses 2 pixel rows per terminal row (▀ half-block), so the pixel
	// height passed to pixterm is rows*2. imageDims keeps the aspect ratio: scaling
	// straight to maxCols x availableRows would stretch the image over the pane.
	cols, rows := imageDims(src, maxCols, availableRows)
	pimg, err := ansimage.NewScaledFromImage(src, rows*2, cols, bg, ansimage.ScaleModeResize, ansimage.NoDithering)
	if err != nil {
		return renderImageBlocksFallback(src, maxCols, availableRows)
	}
	rendered := pimg.Render()

	lines := strings.Split(rendered, "\n")
	// Trim trailing empty line produced by pixterm's trailing \n.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > availableRows {
		lines = lines[:availableRows]
	}
	out := strings.Join(lines, "\n")
	// Pad so the block is exactly availableRows rows (availableRows-1 newlines total).
	if pad := availableRows - len(lines); pad > 0 {
		out += strings.Repeat("\n", pad)
	}
	return out
}

// renderImageBlocksFallback is a pure-Go half-block renderer used when pixterm fails.
func renderImageBlocksFallback(src image.Image, maxCols, maxRows int) string {
	if src == nil || maxCols <= 0 || maxRows <= 0 {
		return strings.Repeat("\n", maxRows-1)
	}
	cols, rows := imageDims(src, maxCols, maxRows)
	pixH := rows * 2
	dst := image.NewRGBA(image.Rect(0, 0, cols, pixH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var sb strings.Builder
	for y := 0; y < pixH; y += 2 {
		for x := 0; x < cols; x++ {
			topC := color.RGBAModel.Convert(dst.At(x, y)).(color.RGBA)
			var botC color.RGBA
			if y+1 < pixH {
				botC = color.RGBAModel.Convert(dst.At(x, y+1)).(color.RGBA)
			}
			fmt.Fprintf(&sb, "\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀",
				topC.R, topC.G, topC.B,
				botC.R, botC.G, botC.B,
			)
		}
		sb.WriteString("\033[0m")
		if y+2 < pixH {
			sb.WriteString("\n")
		}
	}
	// Pad to maxRows rows.
	if pad := maxRows - rows; pad > 0 {
		sb.WriteString(strings.Repeat("\n", pad))
	}
	return sb.String()
}
