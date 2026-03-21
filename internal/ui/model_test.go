package ui

import (
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
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

	newModel, cmd := m.Update(bookmarkMsg{err: nil})
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
