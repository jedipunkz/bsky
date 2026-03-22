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
// Checks known terminal programs and TERM environment variable, with a manual override via BSKY_SIXEL=1.
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

// renderImage renders an image for display in the terminal.
// Uses Sixel protocol when the terminal supports it (detail view only),
// otherwise falls back to pixterm's block-character rendering.
func renderImage(src image.Image, maxCols, maxRows int) string {
	if supportsSixel() {
		if s := renderImageSixel(src, maxCols, maxRows); s != "" {
			return s
		}
	}
	return renderImagePixterm(src, maxCols, maxRows)
}

// renderImageSixel encodes the image using the Sixel graphics protocol.
// The image is scaled to fit within the given column/row budget using approximate
// cell pixel dimensions (8×16 px per cell).
func renderImageSixel(src image.Image, maxCols, maxRows int) string {
	cols, rows := imageDims(src, maxCols, maxRows)
	pixW := cols * 8
	pixH := rows * 16
	dst := image.NewRGBA(image.Rect(0, 0, pixW, pixH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	enc := sixel.NewEncoder(&buf)
	if err := enc.Encode(dst); err != nil {
		return ""
	}
	return buf.String()
}

// renderImagePixterm renders an image using pixterm's ansimage block-character renderer.
// This provides high-quality dithered output on terminals that do not support Sixel.
func renderImagePixterm(src image.Image, maxCols, maxRows int) string {
	bg := color.RGBA{A: 255}
	pimg, err := ansimage.NewScaledFromImage(src, maxRows, maxCols, bg, ansimage.ScaleModeResize, ansimage.NoDithering)
	if err != nil {
		return renderImageBlocks(src, maxCols, maxRows)
	}
	return pimg.Render()
}

// renderImageBlocks renders an image as half-block characters (▀) with ANSI 24-bit color.
// Used as the last-resort fallback.
func renderImageBlocks(src image.Image, maxCols, maxRows int) string {
	if src == nil || maxCols <= 0 || maxRows <= 0 {
		return ""
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
	return sb.String()
}
