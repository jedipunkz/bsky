package ui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	listImageMaxCols = 48
	listImageMaxRows = 20
)

var imgHTTPClient = &http.Client{Timeout: 10 * time.Second}

// iterm2Supported is true when the terminal implements the iTerm2 inline image protocol.
// WezTerm, iTerm2, and Hyper all support it and produce true pixel-quality rendering.
var iterm2Supported = func() bool {
	tp := os.Getenv("TERM_PROGRAM")
	return tp == "WezTerm" || tp == "iTerm.app" || tp == "Hyper"
}()

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

// renderImage dispatches to iTerm2 inline protocol or half-block ANSI based on terminal support.
func renderImage(src image.Image, maxCols, maxRows int) string {
	if iterm2Supported {
		return renderImageITerm2(src, maxCols, maxRows)
	}
	return renderImageBlocks(src, maxCols, maxRows)
}

// renderImageITerm2 sends the image using the iTerm2 inline image protocol.
// The terminal renders it at native resolution — no pixel art.
//
// To keep bubbletea's line counter correct we:
//  1. Emit `rows` newlines  → bubbletea counts these as N lines
//  2. Emit CSI cursor-up N  → cursor moves back to the image start row
//  3. Emit the iTerm2 seq   → terminal draws image, cursor advances N rows to end
func renderImageITerm2(src image.Image, maxCols, maxRows int) string {
	cols, rows := imageDims(src, maxCols, maxRows)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 90}); err != nil {
		// Fallback to half-blocks if encoding fails
		return renderImageBlocks(src, maxCols, maxRows)
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	iterm2 := fmt.Sprintf(
		"\033]1337;File=inline=1;width=%d;height=%d;preserveAspectRatio=0:%s\007",
		cols, rows, b64,
	)

	return strings.Repeat("\n", rows) +
		fmt.Sprintf("\033[%dA", rows) +
		iterm2
}

// renderImageBlocks renders an image as half-block characters (▀) with ANSI 24-bit color.
// Fallback for terminals that do not support the iTerm2 inline image protocol.
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
