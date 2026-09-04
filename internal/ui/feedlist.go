package ui

import "github.com/jedipunkz/bsky/internal/api"

// feedList is a scrollable, paginated list of posts. Timeline tabs, search
// results and profile tabs are the same thing, so they share this state instead
// of each keeping its own set of parallel items/cursor/loading/nextCursor
// fields that have to be kept in sync by hand.
type feedList struct {
	items       []api.FeedItem
	cursor      int
	loading     bool
	loadingMore bool
	nextCursor  string
	err         string
}

// selected returns the post under the cursor, or false when the list is empty.
func (l *feedList) selected() (api.FeedItem, bool) {
	if l.cursor < 0 || l.cursor >= len(l.items) {
		return api.FeedItem{}, false
	}
	return l.items[l.cursor], true
}

func (l *feedList) up() {
	if l.cursor > 0 {
		l.cursor--
	}
}

// down moves the cursor one post down. It reports whether the caller should
// fetch the next page: the cursor is already at the end and one is available.
func (l *feedList) down() bool {
	if l.cursor < len(l.items)-1 {
		l.cursor++
		return false
	}
	if l.loadingMore || l.nextCursor == "" {
		return false
	}
	l.loadingMore = true
	return true
}

func (l *feedList) top() {
	l.cursor = 0
}

func (l *feedList) bottom() {
	if len(l.items) > 0 {
		l.cursor = len(l.items) - 1
	}
}

// set replaces the list with a freshly fetched first page, keeping the cursor
// where it is so a background refresh does not scroll the reader away.
func (l *feedList) set(items []api.FeedItem, cursor string, err error) {
	l.loading = false
	if err != nil {
		l.err = err.Error()
		return
	}
	l.items = items
	l.nextCursor = cursor
	l.err = ""
	l.clamp()
}

// appendPage adds the next page, leaving the cursor on the current post.
func (l *feedList) appendPage(items []api.FeedItem, cursor string, err error) {
	l.loadingMore = false
	if err != nil {
		l.err = err.Error()
		return
	}
	l.items = append(l.items, items...)
	l.nextCursor = cursor
	l.err = ""
}

func (l *feedList) clamp() {
	if l.cursor > len(l.items)-1 {
		l.cursor = len(l.items) - 1
	}
	if l.cursor < 0 {
		l.cursor = 0
	}
}
