package ui

// A terminal keeps a Kitty image on screen until something deletes it, so a
// thumbnail that scrolls away has to be deleted explicitly, and one that moves
// has to be re-placed. Neither is visible from a single frame: it depends on
// what BubbleTea actually writes, which is only the lines that changed. The
// simulation below runs BubbleTea's own diff over consecutive frames and tracks
// the placements a terminal would be left holding.

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/jedipunkz/bsky/internal/api"
)

// flushBytes mirrors bubbletea standard_renderer.flush (alt screen).
func flushBytes(last, next []string, width int) string {
	var b strings.Builder
	b.WriteString("\x1b[H")
	for i := 0; i < len(next); i++ {
		if len(last) > i && last[i] == next[i] {
			if i < len(next)-1 {
				b.WriteByte('\n')
			}
			continue
		}
		line := next[i]
		if width > 0 {
			line = ansi.Truncate(line, width, "")
		}
		if ansi.StringWidth(line) < width {
			line += "\x1b[K"
		}
		b.WriteString(line)
		if i < len(next)-1 {
			b.WriteString("\r\n")
		}
	}
	return b.String()
}

type vterm struct {
	row, col   int
	placements map[uint32]int // image id -> top row
	pending    struct {
		id, rows int
		row, col int
		active   bool
	}
	log []string
}

func newVterm() *vterm { return &vterm{placements: map[uint32]int{}} }

func (v *vterm) consume(s string) {
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\r':
			v.col = 0
			i++
		case c == '\n':
			v.row++
			i++
		case c == 0x1b && i+1 < len(s) && s[i+1] == '[':
			j := i + 2
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == ';' || s[j] == '?') {
				j++
			}
			if j >= len(s) {
				return
			}
			params, final := s[i+2:j], s[j]
			n, _ := strconv.Atoi(strings.SplitN(params, ";", 2)[0])
			if n == 0 {
				n = 1
			}
			switch final {
			case 'A':
				v.row -= n
			case 'B':
				v.row += n
			case 'C':
				v.col += n
			case 'D':
				v.col -= n
			case 'G':
				v.col = n - 1
			case 'H':
				v.row, v.col = 0, 0
			}
			i = j + 1
		case c == 0x1b && i+1 < len(s) && s[i+1] == '_':
			end := strings.Index(s[i:], "\x1b\\")
			if end < 0 {
				return
			}
			body := s[i+2 : i+end]
			v.kitty(body)
			i += end + 2
		case c == 0x1b:
			i++
		default:
			v.col++
			i++
		}
	}
}

func (v *vterm) kitty(body string) {
	ctrl := body
	if k := strings.Index(body, ";"); k >= 0 {
		ctrl = body[:k]
	}
	if !strings.HasPrefix(ctrl, "G") {
		return
	}
	keys := map[string]string{}
	for _, kv := range strings.Split(strings.TrimPrefix(ctrl, "G"), ",") {
		if p := strings.SplitN(kv, "=", 2); len(p) == 2 {
			keys[p[0]] = p[1]
		}
	}
	atoi := func(k string) int { n, _ := strconv.Atoi(keys[k]); return n }
	switch keys["a"] {
	case "d":
		id := uint32(atoi("i"))
		delete(v.placements, id)
		v.log = append(v.log, fmt.Sprintf("delete %d", id))
	case "T":
		v.pending.id, v.pending.rows = atoi("i"), atoi("r")
		v.pending.row, v.pending.col = v.row, v.col
		v.pending.active = true
		if keys["m"] == "0" {
			v.commit()
		}
	default:
		if v.pending.active && keys["m"] == "0" {
			v.commit()
		}
	}
}

func (v *vterm) commit() {
	v.placements[uint32(v.pending.id)] = v.pending.row
	v.log = append(v.log, fmt.Sprintf("place %d at row %d", v.pending.id, v.pending.row))
	v.pending.active = false
}

// intended returns image id -> top row from the frame itself.
func intended(lines []string) map[uint32]int {
	out := map[uint32]int{}
	for i, l := range lines {
		k := strings.Index(l, "\x1b_Ga=T")
		if k < 0 {
			continue
		}
		rest := l[k+2:]
		ctrl := rest[:strings.Index(rest, ";")]
		var id int
		for _, kv := range strings.Split(strings.TrimPrefix(ctrl, "G"), ",") {
			if strings.HasPrefix(kv, "i=") {
				id, _ = strconv.Atoi(strings.TrimPrefix(kv, "i="))
			}
		}
		out[uint32(id)] = i - (thumbRows - 1)
	}
	return out
}

func TestKittyThumbnails_NoGhostsWhileScrolling(t *testing.T) {
	t.Setenv("BSKY_SIXEL", "0")
	t.Setenv("BSKY_KITTY", "1")
	const url = "https://x/t.jpg"
	m := newTestModel()
	m.width, m.height = 80, 24
	m.imageCache[url] = testImage(800, 450)
	for i := range 14 {
		p := api.Post{
			URI:    fmt.Sprintf("at://post/%d", i),
			Author: api.Author{DisplayName: fmt.Sprintf("user%d", i), Handle: fmt.Sprintf("u%d.example", i)},
			Record: api.PostRecord{Text: fmt.Sprintf("post number %d with some text", i)},
		}
		if i%3 == 1 {
			p.Embed = &api.PostEmbedView{Images: []api.EmbedImageView{{Thumb: url}}}
		}
		m.feeds[tabHome].items = append(m.feeds[tabHome].items, api.FeedItem{Post: p})
	}

	// Scroll down through the feed and back up again.
	var path []int
	for c := range 8 {
		path = append(path, c)
	}
	for c := 7; c >= 0; c-- {
		path = append(path, c)
	}

	vt := newVterm()
	var last []string
	for _, cur := range path {
		m.feeds[tabHome].cursor = cur
		lines := strings.Split(m.View(), "\n")
		vt.log = nil
		vt.consume(flushBytes(last, lines, 80))
		last = lines
		want := intended(lines)
		if fmt.Sprint(vt.placements) != fmt.Sprint(want) {
			t.Errorf("cursor=%d: terminal holds %v, frame draws %v (this frame did: %v)",
				cur, vt.placements, want, vt.log)
		}
	}
}
