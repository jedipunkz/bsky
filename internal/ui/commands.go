package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jedipunkz/bsky/internal/api"
)

const (
	feedPageSize   = 50
	searchPageSize = 25
)

// fetchList runs fn off the update loop and delivers the page it returns to the
// list identified by id. more marks a next-page fetch, which appends.
func fetchList(id listID, more bool, fn func() ([]api.FeedItem, string, error)) tea.Cmd {
	return func() tea.Msg {
		items, cursor, err := fn()
		return listMsg{id: id, more: more, items: items, cursor: cursor, err: err}
	}
}

// loadFeed fetches a timeline tab. An empty cursor loads the first page.
func (m *Model) loadFeed(t tab, cursor string) tea.Cmd {
	client := m.client
	return fetchList(timelineList(t), cursor != "", func() ([]api.FeedItem, string, error) {
		switch t {
		case tabSaved:
			// Bookmarks are shown as a single page: no cursor is carried over.
			items, _, err := client.GetBookmarks(feedPageSize, "")
			return items, "", err
		case tabDiscover:
			return client.GetDiscoverFeed(feedPageSize, cursor)
		default:
			return client.GetTimeline(feedPageSize, cursor)
		}
	})
}

func (m *Model) loadSearch(query, cursor string) tea.Cmd {
	client := m.client
	return fetchList(searchList, cursor != "", func() ([]api.FeedItem, string, error) {
		items, next, err := client.SearchPosts(query, searchPageSize, cursor)
		return filterSearchResults(items, query), next, err
	})
}

func (m *Model) loadAuthorFeed(actor string, tabType profileTabType, cursor string) tea.Cmd {
	client := m.client
	return fetchList(profileList(tabType), cursor != "", func() ([]api.FeedItem, string, error) {
		items, next, err := client.GetAuthorFeed(actor, authorFeedFilter(tabType), feedPageSize, cursor)
		return filterReplies(items, tabType), next, err
	})
}

// filterSearchResults drops posts that do not contain the query, which the
// search endpoint returns for fuzzy matches.
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

func authorFeedFilter(tabType profileTabType) string {
	if tabType == profileTabPosts {
		return "posts_no_replies"
	}
	return "posts_with_replies"
}

// filterReplies keeps only replies on the profile's Replies tab; the endpoint
// returns posts and replies together.
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

func (m *Model) fetchProfile(actor string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		profile, err := client.GetProfile(actor)
		return fetchedProfileMsg{profile: profile, err: err}
	}
}

func (m *Model) sendPost(text string, replyTo *api.Post) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if replyTo != nil {
			return postSentMsg{err: client.CreateReply(text, replyTo.URI, replyTo.CID)}
		}
		return postSentMsg{err: client.CreatePost(text)}
	}
}

func (m *Model) toggleLike(post api.Post) tea.Cmd {
	client := m.client
	if post.Viewer.Like != "" {
		return func() tea.Msg {
			return likeMsg{err: client.Unlike(post.Viewer.Like)}
		}
	}
	return func() tea.Msg {
		likeURI, err := client.Like(post.URI, post.CID)
		return likeMsg{err: err, likeURI: likeURI, liked: true}
	}
}

func (m *Model) toggleRepost(post api.Post) tea.Cmd {
	client := m.client
	if post.Viewer.Repost != "" {
		return func() tea.Msg {
			return repostMsg{err: client.Unrepost(post.Viewer.Repost)}
		}
	}
	return func() tea.Msg {
		repostURI, err := client.Repost(post.URI, post.CID)
		return repostMsg{err: err, repostURI: repostURI, reposted: true}
	}
}

func (m *Model) toggleBookmark(post api.Post) tea.Cmd {
	client := m.client
	if m.isBookmarked(post.URI) {
		return func() tea.Msg {
			return bookmarkMsg{err: client.DeleteBookmark(post.URI)}
		}
	}
	return func() tea.Msg {
		return bookmarkMsg{err: client.CreateBookmark(post.URI, post.CID), bookmarked: true}
	}
}

func (m *Model) toggleFollow(profile *api.Profile) tea.Cmd {
	client := m.client
	if profile.Viewer.Following != "" {
		followURI := profile.Viewer.Following
		return func() tea.Msg {
			return followMsg{err: client.Unfollow(followURI)}
		}
	}
	did := profile.DID
	return func() tea.Msg {
		followURI, err := client.Follow(did)
		return followMsg{err: err, followURI: followURI, followed: true}
	}
}

// fetchImageCmd downloads url, unless it is already cached, in flight, or
// known to fail. Returns nil when there is nothing to do.
func (m *Model) fetchImageCmd(url string) tea.Cmd {
	if url == "" {
		return nil
	}
	if _, cached := m.imageCache[url]; cached {
		return nil
	}
	if m.imageLoading[url] {
		return nil
	}
	if _, hasErr := m.imageError[url]; hasErr {
		return nil
	}
	m.imageLoading[url] = true

	return func() tea.Msg {
		img, err := downloadImage(url)
		return imageFetchedMsg{url: url, img: img, err: err}
	}
}

// fetchDetailImageCmd downloads the full-size image of the post open in the
// detail view.
func (m *Model) fetchDetailImageCmd() tea.Cmd {
	return m.fetchImageCmd(detailImageURL(m.detailItem.Post))
}

// thumbWindow is how many posts on either side of the cursor get their
// thumbnail downloaded: enough to cover the screen without pulling a whole
// page of images for posts the reader may never scroll to.
const thumbWindow = 8

// fetchVisibleThumbsCmd downloads the thumbnails of the posts around the
// cursor of the list currently on screen.
func (m *Model) fetchVisibleThumbsCmd() tea.Cmd {
	l := m.thumbList()
	var cmds []tea.Cmd
	for i := max(l.cursor-thumbWindow, 0); i < min(l.cursor+thumbWindow+1, len(l.items)); i++ {
		if c := m.fetchImageCmd(thumbURL(l.items[i].Post)); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

// thumbList returns the list whose thumbnails are currently visible.
func (m *Model) thumbList() *feedList {
	if m.state == stateUserProfile {
		return &m.profileFeeds[m.profileActiveTab]
	}
	l, _ := m.currentList()
	return l
}
