package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jedipunkz/bsky/internal/api"
)

func (m *Model) fetchFeed(t tab) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		var items []api.FeedItem
		var cursor string
		var err error
		if t == tabHome {
			items, cursor, err = client.GetTimeline(50, "")
		} else {
			items, cursor, err = client.GetDiscoverFeed(50, "")
		}
		return fetchedMsg{tab: t, items: items, cursor: cursor, err: err}
	}
}

func (m *Model) loadMoreFeed(t tab, cursor string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		var items []api.FeedItem
		var nextCursor string
		var err error
		if t == tabHome {
			items, nextCursor, err = client.GetTimeline(50, cursor)
		} else {
			items, nextCursor, err = client.GetDiscoverFeed(50, cursor)
		}
		return appendedMsg{tab: t, items: items, cursor: nextCursor, err: err}
	}
}

func (m *Model) sendPost(text string, replyTo *api.Post) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		var err error
		if replyTo != nil {
			err = client.CreateReply(text, replyTo.URI, replyTo.CID)
		} else {
			err = client.CreatePost(text)
		}
		return postSentMsg{err: err}
	}
}

func (m *Model) likePost(uri, cid string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		likeURI, err := client.Like(uri, cid)
		return likeMsg{err: err, likeURI: likeURI, liked: true}
	}
}

func (m *Model) unlikePost(likeURI string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := client.Unlike(likeURI)
		return likeMsg{err: err, liked: false}
	}
}

func (m *Model) repostPost(uri, cid string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		repostURI, err := client.Repost(uri, cid)
		return repostMsg{err: err, repostURI: repostURI, reposted: true}
	}
}

func (m *Model) unrepostPost(repostURI string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := client.Unrepost(repostURI)
		return repostMsg{err: err, reposted: false}
	}
}

func (m *Model) bookmarkPost(item api.FeedItem) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := client.CreateBookmark(item.Post.URI, item.Post.CID)
		return bookmarkMsg{err: err, bookmarked: true}
	}
}

func (m *Model) unbookmarkPost(postURI string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := client.DeleteBookmark(postURI)
		return bookmarkMsg{err: err, bookmarked: false}
	}
}

func (m *Model) searchPosts(query string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		items, cursor, err := client.SearchPosts(query, 25, "")
		return searchMsg{items: items, cursor: cursor, err: err}
	}
}

func (m *Model) loadMoreSearch(query, cursor string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		items, nextCursor, err := client.SearchPosts(query, 25, cursor)
		return appendSearchMsg{items: items, cursor: nextCursor, err: err}
	}
}

func (m *Model) followUser(did string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		followURI, err := client.Follow(did)
		return followMsg{err: err, followURI: followURI, followed: true}
	}
}

func (m *Model) unfollowUser(followURI string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := client.Unfollow(followURI)
		return followMsg{err: err, followed: false}
	}
}

func (m *Model) fetchProfile(actor string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		profile, err := client.GetProfile(actor)
		return fetchedProfileMsg{profile: profile, err: err}
	}
}

func authorFeedFilter(tabType profileTabType) string {
	if tabType == profileTabPosts {
		return "posts_no_replies"
	}
	return "posts_with_replies"
}

func filterReplies(items []api.FeedItem, tabType profileTabType) []api.FeedItem {
	if tabType != profileTabReplies {
		return items
	}
	var replies []api.FeedItem
	for _, item := range items {
		if item.Post.Record.Reply != nil {
			replies = append(replies, item)
		}
	}
	return replies
}

func (m *Model) fetchAuthorFeed(actor string, tabType profileTabType) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		items, cursor, err := client.GetAuthorFeed(actor, authorFeedFilter(tabType), 50, "")
		if err != nil {
			return fetchedAuthorFeedMsg{tabType: tabType, err: err}
		}
		return fetchedAuthorFeedMsg{tabType: tabType, items: filterReplies(items, tabType), cursor: cursor}
	}
}

func (m *Model) loadMoreAuthorFeed(actor string, tabType profileTabType, cursor string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		items, nextCursor, err := client.GetAuthorFeed(actor, authorFeedFilter(tabType), 50, cursor)
		if err != nil {
			return appendedAuthorFeedMsg{tabType: tabType, err: err}
		}
		return appendedAuthorFeedMsg{tabType: tabType, items: filterReplies(items, tabType), cursor: nextCursor}
	}
}

func (m *Model) loadBookmarks() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		items, cursor, err := client.GetBookmarks(50, "")
		if err != nil {
			return fetchedMsg{tab: tabSaved, err: err}
		}
		return fetchedMsg{tab: tabSaved, items: items, cursor: cursor}
	}
}

func fetchDetailImage(url string) tea.Cmd {
	return func() tea.Msg {
		img, err := downloadImage(url)
		if err != nil {
			return imageFetchedMsg{url: url, err: err}
		}
		return imageFetchedMsg{url: url, img: img}
	}
}

func (m *Model) fetchDetailImageCmd() tea.Cmd {
	imgs := m.detailItem.Post.Embed.EmbedImages()
	if len(imgs) == 0 {
		return nil
	}
	imgURL := imgs[0].Fullsize
	if imgURL == "" {
		imgURL = imgs[0].Thumb
	}
	if imgURL == "" {
		return nil
	}
	if _, cached := m.imageCache[imgURL]; cached {
		return nil
	}
	if m.imageLoading[imgURL] {
		return nil
	}
	if _, hasErr := m.imageError[imgURL]; hasErr {
		return nil
	}
	m.imageLoading[imgURL] = true
	return fetchDetailImage(imgURL)
}

func filterSearchResults(items []api.FeedItem, query string) []api.FeedItem {
	q := strings.ToLower(query)
	var filtered []api.FeedItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Post.Record.Text), q) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
