package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
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

// supportsSixel reports whether the current terminal supports the Sixel graphics protocol.
func supportsSixel() bool {
	if os.Getenv("BSKY_SIXEL") == "1" {
		return true
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "WezTerm", "iTerm.app":
		return true
	}
	return strings.Contains(os.Getenv("TERM"), "sixel")
}

// renderImageForView renders src into a string suitable for embedding in BubbleTea's View().
//
// availableRows is the exact number of terminal rows reserved for the image in the layout.
// The returned string always contributes exactly (availableRows-1) newline characters so
// that BubbleTea's line counter matches the visual space the image occupies.
//
// Sixel strategy (when supported):
//
//	  The string is built as:
//	    "\n" × (availableRows-1)          ← placeholder: BubbleTea counts these lines
//	    "\033[<availableRows-1>A"          ← cursor-up: return to the start of the image area
//	    <Sixel DCS>                        ← pixels rendered; cursor ends at bottom of image ✓
//
//	  BubbleTea renders the empty placeholder lines first, then the Sixel line overwrites
//	  them.  On subsequent diff-renders, unchanged lines are skipped, so the Sixel persists.
//
// Block-char fallback:
//
//	pixterm half-block (▀) rendering, padded to exactly availableRows rows.
func renderImageForView(src image.Image, maxCols, availableRows int) string {
	if availableRows < 1 {
		return ""
	}
	if supportsSixel() {
		if s := renderImageSixelView(src, maxCols, availableRows); s != "" {
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
	cols, _ := imageDims(src, maxCols, sixelRows)
	pixW := cols * 8
	pixH := sixelRows * 16
	dst := image.NewRGBA(image.Rect(0, 0, pixW, pixH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	enc := sixel.NewEncoder(&buf)
	if err := enc.Encode(dst); err != nil {
		return ""
	}

	// "\n" × sixelRows  → placeholder lines (BubbleTea counts them)
	// cursor-up sixelRows → back to image area start
	// sixelData          → pixels drawn; cursor ends sixelRows below start ✓
	placeholder := strings.Repeat("\n", sixelRows)
	cursorUp := fmt.Sprintf("\033[%dA", sixelRows)
	return placeholder + cursorUp + buf.String()
}

// renderImageBlockView renders src as half-block characters (▀) with ANSI 24-bit colour,
// padded/trimmed to exactly availableRows terminal rows ((availableRows-1) newlines).
func renderImageBlockView(src image.Image, maxCols, availableRows int) string {
	bg := color.RGBA{A: 255}
	// NoDithering uses 2 pixel rows per terminal row (▀ half-block).
	// Pass availableRows*2 as pixel height so output is exactly availableRows terminal rows.
	pimg, err := ansimage.NewScaledFromImage(src, availableRows*2, maxCols, bg, ansimage.ScaleModeResize, ansimage.NoDithering)
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
