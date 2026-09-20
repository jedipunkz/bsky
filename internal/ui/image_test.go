package ui

import (
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jedipunkz/bsky/internal/api"
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

// A feed item must keep the same geometry whether its thumbnail has arrived or
// not, so the list does not reflow when a download finishes.
func TestRenderFeedItem_ThumbnailGeometryIsStable(t *testing.T) {
	t.Setenv("BSKY_SIXEL", "0")
	t.Setenv("BSKY_KITTY", "0")
	m := newTestModel()
	const url = "https://cdn.example/thumb.jpg"
	item := api.FeedItem{Post: api.Post{
		Record: api.PostRecord{Text: "a post with a picture"},
		Embed:  &api.PostEmbedView{Images: []api.EmbedImageView{{Thumb: url}}},
	}}

	pending := m.renderFeedItem(item, false, 60)
	m.imageCache[url] = testImage(400, 200)
	loaded := m.renderFeedItem(item, false, 60)

	if got, want := lipgloss.Height(loaded), lipgloss.Height(pending); got != want {
		t.Errorf("height with thumbnail = %d, without = %d", got, want)
	}
	if h := lipgloss.Height(loaded); h < thumbRows {
		t.Errorf("height = %d, want at least thumbRows (%d)", h, thumbRows)
	}
	if got, want := lipgloss.Width(loaded), lipgloss.Width(pending); got != want {
		t.Errorf("width with thumbnail = %d, without = %d", got, want)
	}
	// The thumbnail must actually be drawn: block pixels carry 24-bit colour.
	if !strings.Contains(loaded, "\x1b[48;2;") {
		t.Error("loaded item has no block pixels")
	}
}

// The Sixel thumbnail draws its pixels on the last line of the block and must
// leave the cursor on that same line, or every post below it shifts.
func TestRenderThumbBlock_SixelRowAccounting(t *testing.T) {
	t.Setenv("BSKY_SIXEL", "1")
	const blockRows = 5
	out := renderThumbBlock(testImage(400, 200), 40, blockRows, 99)

	if got := strings.Count(out, "\n"); got != blockRows-1 {
		t.Errorf("block spans %d newlines, want %d", got, blockRows-1)
	}
	if !strings.Contains(out, "\x1bP") {
		t.Fatal("no Sixel data in block")
	}
	if lipgloss.Width(out) != 0 {
		t.Errorf("block last line measures %d cells wide, want 0", lipgloss.Width(out))
	}
	// cursor-up to the top of the block, pixels, then back down to the last line.
	cols, rows := imageDims(testImage(400, 200), 40, blockRows-1)
	_ = cols
	wantUp := fmt.Sprintf("\x1b[%dA", blockRows-1)
	if !strings.Contains(out, wantUp) {
		t.Errorf("missing cursor-up %q", wantUp)
	}
	if back := blockRows - 1 - rows; back > 0 && !strings.Contains(out, fmt.Sprintf("\x1b[%dB", back)) {
		t.Errorf("missing cursor-down %d", back)
	}
}

// BubbleTea skips repainting a line that is byte-identical to the last frame.
// A thumbnail's rows hold nothing but spaces, so without a per-image tag two
// scroll positions can render such a row identically, the row is never
// repainted, and the pixels drawn on it stay on screen on top of whatever moved
// into their place. The seeds below are layouts where that happens.
func TestThumbRows_RepaintWhenTheImageMoves(t *testing.T) {
	t.Setenv("BSKY_SIXEL", "1")
	const url = "https://cdn.example/thumb.jpg"

	pixelRows := func(lines []string) []int {
		var rows []int
		for i, l := range lines {
			if strings.Contains(l, "\x1bP") { // Sixel DCS
				rows = append(rows, i)
			}
		}
		return rows
	}
	covers := func(rows []int, r int) bool {
		for _, e := range rows {
			if r > e-thumbRows && r <= e {
				return true
			}
		}
		return false
	}

	for _, seed := range []int64{8, 27, 54} {
		rng := rand.New(rand.NewSource(seed))
		m := newTestModel()
		m.width, m.height = 80, 30
		m.imageCache[url] = testImage(800, 450)
		for i := range 15 {
			post := api.Post{
				URI:    fmt.Sprintf("at://post/%d", i),
				Author: api.Author{DisplayName: fmt.Sprintf("user%d", i), Handle: fmt.Sprintf("u%d.example", i)},
				Record: api.PostRecord{Text: strings.Repeat("word ", 1+rng.Intn(40))},
			}
			if rng.Intn(3) == 0 {
				post.Embed = &api.PostEmbedView{Images: []api.EmbedImageView{{Thumb: url}}}
			}
			m.feeds[tabHome].items = append(m.feeds[tabHome].items, api.FeedItem{Post: post})
		}
		render := func(cursor int) []string {
			m.feeds[tabHome].cursor = cursor
			return strings.Split(m.View(), "\n")
		}

		for cursor := range 10 {
			before, after := render(cursor), render(cursor+1)
			for _, e := range pixelRows(before) {
				for r := e - thumbRows + 1; r <= e; r++ {
					if r >= len(after) || covers(pixelRows(after), r) {
						continue // off screen, or painted over by the redraw
					}
					if before[r] == after[r] {
						t.Errorf("seed %d, cursor %d->%d: row %d is unchanged, so the pixels on it are never erased",
							seed, cursor, cursor+1, r)
					}
				}
			}
		}
	}
}

// A Kitty thumbnail must delete its previous placement before making a new one:
// a placement that is not replaced stays where the post used to be, and the
// picture shows up twice.
func TestRenderThumbBlock_KittyDeletesBeforePlacing(t *testing.T) {
	t.Setenv("BSKY_SIXEL", "0")
	t.Setenv("BSKY_KITTY", "1")
	out := renderThumbBlock(testImage(400, 200), 40, thumbRows, 4242)

	del, place := strings.Index(out, "a=d"), strings.Index(out, "a=T")
	switch {
	case del < 0:
		t.Error("no delete before the placement")
	case place < 0:
		t.Fatal("no Kitty placement in block")
	case del > place:
		t.Error("delete comes after the placement")
	}
	if !strings.Contains(out, "i=4242") {
		t.Error("placement does not carry the image id")
	}
}

// A thumbnail that scrolls off screen has to be deleted, and the delete has to
// be repeated: BubbleTea keeps only the newest frame, so one emitted once can
// be dropped before it reaches the terminal.
func TestReapThumbs_DeletesDepartedImagesRepeatedly(t *testing.T) {
	t.Setenv("BSKY_SIXEL", "0")
	t.Setenv("BSKY_KITTY", "1")
	const url = "https://cdn.example/thumb.jpg"
	m := newTestModel()
	m.width, m.height = 80, 24
	m.imageCache[url] = testImage(800, 450)
	for i := range 12 {
		post := api.Post{
			URI:    fmt.Sprintf("at://post/%d", i),
			Author: api.Author{DisplayName: fmt.Sprintf("user%d", i), Handle: fmt.Sprintf("u%d.example", i)},
			Record: api.PostRecord{Text: fmt.Sprintf("post number %d", i)},
		}
		if i == 0 {
			post.Embed = &api.PostEmbedView{Images: []api.EmbedImageView{{Thumb: url}}}
		}
		m.feeds[tabHome].items = append(m.feeds[tabHome].items, api.FeedItem{Post: post})
	}
	want := fmt.Sprintf("i=%d", thumbID(m.feeds[tabHome].items[0].Post, url))

	// The reaper writes in front of the frame; a thumbnail's own delete-then-place
	// sits further down, on the line that draws it.
	head := func() string { return strings.SplitN(m.View(), "\n", 2)[0] }

	m.feeds[tabHome].cursor = 0
	if strings.Contains(head(), want) {
		t.Error("deleted a thumbnail that is on screen")
	}
	// Scroll past it: the cursor post is rendered at the top, so the image is gone.
	m.feeds[tabHome].cursor = 4
	for i := range deleteFrames {
		if h := head(); !strings.Contains(h, "a=d,d=I,"+want) {
			t.Errorf("frame %d after it left the screen carries no delete for it", i)
		}
	}
	if strings.Contains(head(), want) {
		t.Error("delete is still repeated after deleteFrames frames")
	}
}
