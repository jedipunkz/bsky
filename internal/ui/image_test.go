package ui

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	return img
}

func TestImageDims_PreservesAspectRatio(t *testing.T) {
	// A 2:1 landscape image in a 100x40 pane must not fill all 40 rows:
	// cells are ~2:1, so 100 cols of a 2:1 image is 25 rows.
	cols, rows := imageDims(testImage(400, 200), 100, 40)
	if cols != 100 || rows != 25 {
		t.Errorf("imageDims = (%d, %d), want (100, 25)", cols, rows)
	}
}

func TestRenderImageBlockView_ExactRowsAndWidth(t *testing.T) {
	const maxCols, rows = 60, 20
	out := renderImageBlockView(testImage(400, 200), maxCols, rows)

	lines := strings.Split(out, "\n")
	if len(lines) != rows {
		t.Errorf("got %d rows, want %d", len(lines), rows)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > maxCols {
			t.Errorf("line %d is %d cells wide, want <= %d", i, w, maxCols)
		}
	}

	// The image must keep its aspect ratio instead of being stretched over the pane.
	var drawn int
	for _, l := range lines {
		if ansi.StringWidth(l) > 0 {
			drawn++
		}
	}
	// 60 cols of a 2:1 image is ~15 rows, not the full 20-row pane.
	if drawn < 14 || drawn > 15 {
		t.Errorf("drawn rows = %d, want ~15 (aspect-preserved)", drawn)
	}
}

func TestSupportsSixel_OptIn(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "WezTerm")
	t.Setenv("TERM", "xterm-256color")
	if supportsSixel() {
		t.Error("Sixel must stay off unless BSKY_SIXEL=1: TERM_PROGRAM survives multiplexers that drop Sixel")
	}
	t.Setenv("BSKY_SIXEL", "1")
	if !supportsSixel() {
		t.Error("BSKY_SIXEL=1 must enable Sixel")
	}
}
