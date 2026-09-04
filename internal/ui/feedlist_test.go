package ui

import (
	"errors"
	"testing"

	"github.com/jedipunkz/bsky/internal/api"
)

func items(uris ...string) []api.FeedItem {
	out := make([]api.FeedItem, len(uris))
	for i, u := range uris {
		out[i] = api.FeedItem{Post: api.Post{URI: u}}
	}
	return out
}

func TestFeedList_DownLoadsNextPageOnlyAtTheEnd(t *testing.T) {
	l := feedList{items: items("a", "b"), nextCursor: "next"}

	if l.down() {
		t.Error("moving within the list must not trigger a fetch")
	}
	if l.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", l.cursor)
	}

	if !l.down() || !l.loadingMore {
		t.Fatal("reaching the end with a cursor available must trigger a fetch")
	}
	if l.down() {
		t.Error("a second fetch must not start while one is in flight")
	}

	l.loadingMore, l.nextCursor = false, ""
	if l.down() {
		t.Error("the last page must not trigger a fetch")
	}
}

func TestFeedList_SelectedAndBounds(t *testing.T) {
	var l feedList
	if _, ok := l.selected(); ok {
		t.Error("an empty list has no selection")
	}

	l.set(items("a", "b", "c"), "next", nil)
	l.bottom()
	if item, ok := l.selected(); !ok || item.Post.URI != "c" {
		t.Errorf("selected = %v, %v; want c", item.Post.URI, ok)
	}
	l.up()
	l.top()
	if l.cursor != 0 {
		t.Errorf("cursor = %d, want 0", l.cursor)
	}
	l.up()
	if l.cursor != 0 {
		t.Errorf("cursor = %d, want it to stay at 0", l.cursor)
	}
}

func TestFeedList_SetKeepsCursorAndClampsIt(t *testing.T) {
	l := feedList{items: items("a", "b", "c"), cursor: 2, loading: true}

	l.set(items("a", "b", "c", "d"), "next", nil)
	if l.cursor != 2 {
		t.Errorf("cursor = %d, want the reader to stay on post 2", l.cursor)
	}
	if l.loading {
		t.Error("set must clear the loading flag")
	}

	l.set(items("a"), "", nil)
	if l.cursor != 0 {
		t.Errorf("cursor = %d, want it clamped into the shorter list", l.cursor)
	}
}

func TestFeedList_ErrorsKeepExistingItems(t *testing.T) {
	l := feedList{items: items("a"), cursor: 0, loading: true, loadingMore: true}

	l.set(nil, "", errors.New("boom"))
	if len(l.items) != 1 || l.err != "boom" || l.loading {
		t.Errorf("set with an error = %+v", l)
	}

	l.appendPage(nil, "", errors.New("bang"))
	if len(l.items) != 1 || l.err != "bang" || l.loadingMore {
		t.Errorf("appendPage with an error = %+v", l)
	}

	l.appendPage(items("b"), "next2", nil)
	if len(l.items) != 2 || l.err != "" || l.nextCursor != "next2" {
		t.Errorf("appendPage = %+v", l)
	}
}
