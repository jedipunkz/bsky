package ui

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	_ "golang.org/x/image/webp"
	"net/http"
	"strings"
	"time"
)

const (
	listImageCols = 24
	listImageRows = 5
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

func resizeImageNN(src image.Image, newW, newH int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	srcBounds := src.Bounds()
	srcW := srcBounds.Max.X - srcBounds.Min.X
	srcH := srcBounds.Max.Y - srcBounds.Min.Y
	if srcW == 0 || srcH == 0 {
		return dst
	}
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := srcBounds.Min.X + x*srcW/newW
			srcY := srcBounds.Min.Y + y*srcH/newH
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

// renderImageBlocks renders an image as half-block characters (▀) with ANSI 24-bit color.
// Each terminal row shows 2 pixel rows: foreground = top pixel, background = bottom pixel.
func renderImageBlocks(src image.Image, cols, rows int) string {
	if src == nil || cols <= 0 || rows <= 0 {
		return ""
	}
	pixH := rows * 2
	resized := resizeImageNN(src, cols, pixH)
	var sb strings.Builder
	for y := 0; y < pixH; y += 2 {
		for x := 0; x < cols; x++ {
			topC := color.RGBAModel.Convert(resized.At(x, y)).(color.RGBA)
			var botC color.RGBA
			if y+1 < pixH {
				botC = color.RGBAModel.Convert(resized.At(x, y+1)).(color.RGBA)
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
