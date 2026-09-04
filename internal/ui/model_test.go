package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/jedipunkz/bsky/internal/api"
)

func newTestModel() *Model {
	applyTheme("tokyonight")
	ta := textarea.New()
	return &Model{
		client:  nil,
		compose: ta,
		width:   80,
		height:  24,
	}
}

func TestModelUpdate_BookmarkKeyReturnsCmd(t *testing.T) {
	m := newTestModel()
	m.state = stateDetail
	m.detailItem = api.FeedItem{
		Post: api.Post{
			URI:    "at://did:plc:abc123/app.bsky.feed.post/rkey1",
			CID:    "cid1",
			Author: api.Author{Handle: "testuser.bsky.social"},
			Record: api.PostRecord{Text: "Hello!"},
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	if cmd == nil {
		t.Fatal("expected a command after pressing b, got nil")
	}
}

func TestModelUpdate_BookmarkMsgSuccess(t *testing.T) {
	m := newTestModel()
	m.state = stateDetail

	newModel, cmd := m.Update(bookmarkMsg{err: nil, bookmarked: true})
	m = newModel.(*Model)

	if m.statusMsg != "Bookmarked!" {
		t.Errorf("expected statusMsg 'Bookmarked!', got %q", m.statusMsg)
	}
	if cmd == nil {
		t.Error("expected loadBookmarks command after successful bookmarkMsg")
	}
}

func TestModelUpdate_BookmarkMsgError(t *testing.T) {
	m := newTestModel()
	m.state = stateDetail

	testErr := errors.New("api error")
	newModel, cmd := m.Update(bookmarkMsg{err: testErr})
	m = newModel.(*Model)

	if m.statusMsg != "Bookmark failed: api error" {
		t.Errorf("expected error statusMsg, got %q", m.statusMsg)
	}
	if cmd != nil {
		t.Error("expected nil command after failed bookmarkMsg")
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	cases := []struct {
		in   string
		want string
	}{
		{now.Add(-30 * time.Second).Format(time.RFC3339), "now"},
		{now.Add(-5 * time.Minute).Format(time.RFC3339), "5m"},
		{now.Add(-3 * time.Hour).Format(time.RFC3339), "3h"},
		{now.Add(-50 * time.Hour).Format(time.RFC3339), "2d"},
		{"not-a-time", ""},
	}
	for _, c := range cases {
		if got := relativeTime(c.in); got != c.want {
			t.Errorf("relativeTime(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRenderPostMeta_OmitsZeroCounters(t *testing.T) {
	m := newTestModel()

	if got := m.renderPostMeta(api.Post{}); got != "" {
		t.Errorf("expected empty meta for a post with no activity, got %q", got)
	}

	post := api.Post{LikeCount: 3, Viewer: api.PostViewer{Like: "at://like"}}
	got := m.renderPostMeta(post)
	if !strings.Contains(got, "♥ 3") {
		t.Errorf("expected like counter in meta, got %q", got)
	}
	if strings.Contains(got, "↺") || strings.Contains(got, "✦") {
		t.Errorf("expected zero counters to be omitted, got %q", got)
	}
}

func TestWrapText_WrapsCJKByDisplayWidth(t *testing.T) {
	// 10 full-width runes = 20 cells; wrapping at 10 cells must produce 2 lines.
	lines := strings.Split(wrapText(strings.Repeat("あ", 10), 10), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), lines)
	}
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > 10 {
			t.Errorf("line width %d exceeds limit 10: %q", w, l)
		}
	}
}
