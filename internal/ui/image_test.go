package ui

import (
	"fmt"
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

func TestSupportsKitty_AutoDetect(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"wezterm", map[string]string{"TERM_PROGRAM": "WezTerm"}, true},
		{"ghostty term", map[string]string{"TERM": "xterm-ghostty"}, true},
		{"kitty window id", map[string]string{"KITTY_WINDOW_ID": "1"}, true},
		{"plain xterm", map[string]string{"TERM": "xterm-256color"}, false},
		// tmux and zellij do not forward the APC sequence; herdr does, so it is not listed.
		{"inside tmux", map[string]string{"TERM_PROGRAM": "WezTerm", "TMUX": "/tmp/tmux-0/default"}, false},
		{"inside zellij", map[string]string{"TERM_PROGRAM": "WezTerm", "ZELLIJ": "0"}, false},
		{"forced off", map[string]string{"TERM_PROGRAM": "WezTerm", "BSKY_KITTY": "0"}, false},
		{"forced on", map[string]string{"TERM": "xterm-256color", "BSKY_KITTY": "1"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"TERM", "TERM_PROGRAM", "KITTY_WINDOW_ID", "TMUX", "ZELLIJ", "BSKY_KITTY"} {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if got := supportsKitty(); got != tc.want {
				t.Errorf("supportsKitty() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRenderImageKittyView_RowAccountingAndProtocol(t *testing.T) {
	const maxCols, availableRows = 60, 20
	out := renderImageKittyView(testImage(400, 200), maxCols, availableRows)
	if out == "" {
		t.Fatal("renderImageKittyView returned empty string")
	}

	// The layout contract: exactly (availableRows-1) newlines, no visible cells.
	if got := strings.Count(out, "\n"); got != availableRows-1 {
		t.Errorf("newline count = %d, want %d", got, availableRows-1)
	}
	if w := ansi.StringWidth(out); w != 0 {
		t.Errorf("rendered width = %d cells, want 0 (escape sequences only)", w)
	}

	// q=2 keeps the terminal's replies off stdin, C=1 keeps the cursor under our control,
	// and non-zero i=/p= make a repaint replace the placement instead of stacking one.
	for _, want := range []string{"\033_Ga=T,f=100,", "q=2,", "C=1,", "i=8151,", "p=1,"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in the Kitty command", want)
		}
	}

	// Cursor moves up and back down by the same number of rows.
	up := strings.Index(out, "\033[")
	if up < 0 {
		t.Fatal("no cursor movement found")
	}
	var upRows, downRows int
	if _, err := fmt.Sscanf(out[up:], "\033[%dA", &upRows); err != nil {
		t.Fatalf("cursor-up parse: %v", err)
	}
	down := strings.LastIndex(out, "\033[")
	if _, err := fmt.Sscanf(out[down:], "\033[%dB", &downRows); err != nil {
		t.Fatalf("cursor-down parse: %v", err)
	}
	if upRows != downRows || upRows < 1 {
		t.Errorf("cursor up %d rows, down %d rows; want equal and >= 1", upRows, downRows)
	}
}

func TestRenderImageKittyView_ChunksPayload(t *testing.T) {
	out := renderImageKittyView(testImage(800, 800), 200, 60)
	// A large image must be split, every chunk but the last flagged m=1.
	chunks := strings.Count(out, "\033_G")
	if chunks < 2 {
		t.Fatalf("got %d Kitty chunks, want the payload split into several", chunks)
	}
	if got := strings.Count(out, "m=1;"); got != chunks-1 {
		t.Errorf("m=1 appears %d times across %d chunks, want %d", got, chunks, chunks-1)
	}
	if !strings.Contains(out, "m=0;") {
		t.Error("the final chunk must be flagged m=0")
	}
}
