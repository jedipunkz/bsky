package ui

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
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
// We solve for rows = imgH * cols / (imgW * 2) and clamp to maxCols/maxRows.
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

	// If rows exceeds maxRows, scale cols down proportionally.
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

// renderImageBlocks renders an image as half-block characters (▀) with ANSI 24-bit color.
// Each terminal row shows 2 pixel rows: foreground = top pixel, background = bottom pixel.
// Dimensions are derived from the image's aspect ratio within maxCols × maxRows.
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
