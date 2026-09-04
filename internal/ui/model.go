package ui

import (
	"fmt"
	"image"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jedipunkz/bsky/internal/api"
)

type tab int

const (
	tabHome tab = iota
	tabDiscover
	tabSaved
	tabCount
)

type state int

const (
	stateTimeline state = iota
	stateCompose
	stateDetail
	stateSearch
	stateUserProfile
)

type profileTabType int

const (
	profileTabPosts profileTabType = iota
	profileTabReplies
	profileTabCount
)

// listID addresses one of the model's feed lists, so a command started now can
// deliver its page to the right list later.
type listID struct {
	kind listKind
	idx  int
}

type listKind int

const (
	listTimeline listKind = iota
	listProfile
	listSearch
)

func timelineList(t tab) listID            { return listID{kind: listTimeline, idx: int(t)} }
func profileList(pt profileTabType) listID { return listID{kind: listProfile, idx: int(pt)} }

var searchList = listID{kind: listSearch}

// listMsg carries a fetched page back to the list it was requested for. more
// marks a next-page fetch, which appends instead of replacing.
type listMsg struct {
	id     listID
	more   bool
	items  []api.FeedItem
	cursor string
	err    error
}

type imageFetchedMsg struct {
	url string
	img image.Image
	err error
}

type postSentMsg struct {
	err error
}

type likeMsg struct {
	err     error
	likeURI string
	liked   bool
}

type repostMsg struct {
	err       error
	repostURI string
	reposted  bool
}

type bookmarkMsg struct {
	err        error
	bookmarked bool
}

type followMsg struct {
	err       error
	followURI string
	followed  bool
}

type fetchedProfileMsg struct {
	profile *api.Profile
	err     error
}

type Model struct {
	client    *api.Client
	width     int
	height    int
	activeTab tab
	state     state
	prevState state

	feeds [tabCount]feedList

	search      feedList
	searchInput textinput.Model
	searchQuery string
	inSearch    bool

	detailItem api.FeedItem
	replyTo    *api.Post

	compose     textarea.Model
	composeErr  string
	postSuccess bool

	statusMsg string

	profileActor     string
	profileData      *api.Profile
	profileLoading   bool
	profileActiveTab profileTabType
	profileFeeds     [profileTabCount]feedList
	profilePrevState state

	imageCache   map[string]image.Image // URL -> decoded image (rendered at display time)
	imageLoading map[string]bool        // URL -> loading in progress
	imageError   map[string]string      // URL -> error message
}

func New(client *api.Client, theme string) *Model {
	applyTheme(theme)

	ta := textarea.New()
	ta.Placeholder = "What's on your mind? (Ctrl+S to post, Esc to cancel)"
	ta.CharLimit = 300
	ta.SetWidth(60)
	ta.SetHeight(5)
	ta.Focus()

	si := textinput.New()
	si.Placeholder = "Search posts..."
	si.CharLimit = 100

	return &Model{
		client:       client,
		compose:      ta,
		searchInput:  si,
		imageCache:   make(map[string]image.Image),
		imageLoading: make(map[string]bool),
		imageError:   make(map[string]string),
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadFeed(tabHome, ""),
		m.loadFeed(tabDiscover, ""),
		m.loadFeed(tabSaved, ""),
	)
}

// list resolves a listID to the list it addresses.
func (m *Model) list(id listID) *feedList {
	switch id.kind {
	case listProfile:
		return &m.profileFeeds[id.idx]
	case listSearch:
		return &m.search
	default:
		return &m.feeds[id.idx]
	}
}

// currentList returns the list the timeline view is scrolling: the search
// results when a search is active, otherwise the active tab.
func (m *Model) currentList() (*feedList, listID) {
	if m.inSearch {
		return &m.search, searchList
	}
	return &m.feeds[m.activeTab], timelineList(m.activeTab)
}

// loadMore fetches the next page of the given list.
func (m *Model) loadMore(id listID, cursor string) tea.Cmd {
	switch id.kind {
	case listProfile:
		return m.loadAuthorFeed(m.profileActor, profileTabType(id.idx), cursor)
	case listSearch:
		return m.loadSearch(m.searchQuery, cursor)
	default:
		return m.loadFeed(tab(id.idx), cursor)
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.compose.SetWidth(m.width/2 - 8)
		return m, nil

	case listMsg:
		l := m.list(msg.id)
		if msg.more {
			l.appendPage(msg.items, msg.cursor, msg.err)
		} else {
			l.set(msg.items, msg.cursor, msg.err)
		}
		if msg.id.kind == listSearch {
			m.reportSearch(msg)
		}
		return m, nil

	case imageFetchedMsg:
		delete(m.imageLoading, msg.url)
		if msg.err != nil {
			m.imageError[msg.url] = msg.err.Error()
		} else {
			m.imageCache[msg.url] = msg.img
		}
		return m, nil

	case postSentMsg:
		if msg.err != nil {
			m.composeErr = msg.err.Error()
			return m, nil
		}
		m.state = m.prevState
		m.compose.Reset()
		m.composeErr = ""
		m.replyTo = nil
		m.postSuccess = true
		m.statusMsg = "Post sent!"
		return m, tea.Batch(
			m.loadFeed(tabHome, ""),
			m.loadFeed(tabDiscover, ""),
		)

	case likeMsg:
		if msg.err != nil {
			m.statusMsg = "Like failed: " + msg.err.Error()
			return m, nil
		}
		post := &m.detailItem.Post
		if msg.liked {
			m.statusMsg = "Liked!"
			post.LikeCount++
			post.Viewer.Like = msg.likeURI
		} else {
			m.statusMsg = "Unliked!"
			post.LikeCount = decrement(post.LikeCount)
			post.Viewer.Like = ""
		}
		m.syncDetailItemToFeed()
		return m, nil

	case repostMsg:
		if msg.err != nil {
			m.statusMsg = "Repost failed: " + msg.err.Error()
			return m, nil
		}
		post := &m.detailItem.Post
		if msg.reposted {
			m.statusMsg = "Reposted!"
			post.RepostCount++
			post.Viewer.Repost = msg.repostURI
		} else {
			m.statusMsg = "Unreposted!"
			post.RepostCount = decrement(post.RepostCount)
			post.Viewer.Repost = ""
		}
		m.syncDetailItemToFeed()
		return m, nil

	case bookmarkMsg:
		if msg.err != nil {
			m.statusMsg = "Bookmark failed: " + msg.err.Error()
			return m, nil
		}
		if msg.bookmarked {
			m.statusMsg = "Bookmarked!"
		} else {
			m.statusMsg = "Unbookmarked!"
		}
		return m, m.loadFeed(tabSaved, "")

	case followMsg:
		if msg.err != nil {
			m.statusMsg = "Follow failed: " + msg.err.Error()
			return m, nil
		}
		if msg.followed {
			m.statusMsg = "Followed!"
		} else {
			m.statusMsg = "Unfollowed!"
		}
		if m.profileData != nil {
			m.profileData.Viewer.Following = msg.followURI
			if msg.followed {
				m.profileData.FollowersCount++
			} else {
				m.profileData.FollowersCount = decrement(m.profileData.FollowersCount)
			}
		}
		return m, nil

	case fetchedProfileMsg:
		m.profileLoading = false
		if msg.err == nil {
			m.profileData = msg.profile
		}
		return m, nil
	}

	switch m.state {
	case stateCompose:
		return m.updateCompose(msg)
	case stateDetail:
		return m.updateDetail(msg)
	case stateSearch:
		return m.updateSearch(msg)
	case stateUserProfile:
		return m.updateUserProfile(msg)
	default:
		return m.updateTimeline(msg)
	}
}

func decrement(n int) int {
	if n > 0 {
		return n - 1
	}
	return 0
}

// reportSearch turns the outcome of a search into a status message and, on the
// first page, switches the timeline over to the results.
func (m *Model) reportSearch(msg listMsg) {
	if msg.err != nil {
		if !msg.more {
			m.inSearch = false
			m.statusMsg = "Search failed: " + msg.err.Error()
		}
		return
	}
	if !msg.more {
		m.search.top()
		m.inSearch = true
	}
	m.statusMsg = fmt.Sprintf("Search: %q (%d results)", m.searchQuery, len(m.search.items))
}

func (m *Model) exitSearch() {
	m.search = feedList{}
	m.searchQuery = ""
	m.inSearch = false
	m.statusMsg = ""
}

func (m *Model) startCompose(replyTo *api.Post, from state) {
	m.statusMsg = ""
	m.replyTo = replyTo
	m.prevState = from
	m.state = stateCompose
	m.composeErr = ""
	m.compose.Reset()
	m.compose.Focus()
}

func (m *Model) updateTimeline(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	m.postSuccess = false
	list, id := m.currentList()

	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "q":
		if !m.inSearch {
			return m, tea.Quit
		}
		m.exitSearch()

	case "esc":
		if m.inSearch {
			m.exitSearch()
		}

	case "j":
		m.statusMsg = ""
		if list.down() {
			return m, m.loadMore(id, list.nextCursor)
		}

	case "k":
		m.statusMsg = ""
		list.up()

	case "g":
		m.statusMsg = ""
		list.top()

	case "G":
		m.statusMsg = ""
		list.bottom()

	case "h":
		if !m.inSearch && m.activeTab > 0 {
			m.statusMsg = ""
			m.activeTab--
		}

	case "l":
		if !m.inSearch && m.activeTab < tabCount-1 {
			m.statusMsg = ""
			m.activeTab++
		}

	case "enter":
		m.statusMsg = ""
		if item, ok := list.selected(); ok {
			m.detailItem = item
			m.state = stateDetail
			return m, m.fetchDetailImageCmd()
		}

	case "u":
		m.statusMsg = ""
		if item, ok := list.selected(); ok {
			return m.openUserProfile(item.Post.Author, stateTimeline)
		}

	case "c":
		m.startCompose(nil, stateTimeline)

	case "s":
		m.state = stateSearch
		m.searchInput.SetValue("")
		m.searchInput.Focus()

	case "r":
		if !m.inSearch {
			m.statusMsg = ""
			list.loading = true
			list.top()
			return m, m.loadFeed(m.activeTab, "")
		}
	}
	return m, nil
}

func (m *Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	post := m.detailItem.Post

	switch key.String() {
	case "esc", "q", "enter":
		m.state = stateTimeline
		m.statusMsg = ""
		return m, tea.ClearScreen

	case "l":
		return m, m.toggleLike(post)

	case "r":
		return m, m.toggleRepost(post)

	case "b":
		return m, m.toggleBookmark(post)

	case "c":
		m.startCompose(&post, stateDetail)

	case "u":
		return m.openUserProfile(post.Author, stateDetail)
	}
	return m, nil
}

func (m *Model) updateCompose(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.state = m.prevState
			m.replyTo = nil
			m.composeErr = ""
			return m, nil
		case "ctrl+s":
			text := strings.TrimSpace(m.compose.Value())
			switch {
			case text == "":
				m.composeErr = "Post cannot be empty"
				return m, nil
			case len([]rune(text)) > 300:
				m.composeErr = "Post exceeds 300 characters"
				return m, nil
			}
			return m, m.sendPost(text, m.replyTo)
		}
	}
	var cmd tea.Cmd
	m.compose, cmd = m.compose.Update(msg)
	return m, cmd
}

func (m *Model) updateUserProfile(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	list := &m.profileFeeds[m.profileActiveTab]

	switch key.String() {
	case "q", "esc":
		m.state = m.profilePrevState
		m.statusMsg = ""

	case "f":
		if m.profileData == nil {
			return m, nil
		}
		return m, m.toggleFollow(m.profileData)

	case "h":
		if m.profileActiveTab > 0 {
			m.profileActiveTab--
		}

	case "l":
		if m.profileActiveTab < profileTabCount-1 {
			m.profileActiveTab++
		}

	case "j":
		if list.down() {
			return m, m.loadMore(profileList(m.profileActiveTab), list.nextCursor)
		}

	case "k":
		list.up()

	case "g":
		list.top()

	case "G":
		list.bottom()
	}
	return m, nil
}

func (m *Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.state = stateTimeline
			return m, nil
		case "enter":
			query := strings.TrimSpace(m.searchInput.Value())
			if query == "" {
				return m, nil
			}
			m.searchQuery = query
			m.search.loading = true
			m.inSearch = false
			m.state = stateTimeline
			return m, m.loadSearch(query, "")
		}
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *Model) openUserProfile(author api.Author, prevState state) (tea.Model, tea.Cmd) {
	m.profileActor = author.Handle
	m.profileData = nil
	m.profileLoading = true
	m.profileActiveTab = profileTabPosts
	m.profileFeeds = [profileTabCount]feedList{{loading: true}, {loading: true}}
	m.profilePrevState = prevState
	m.state = stateUserProfile

	return m, tea.Batch(
		m.fetchProfile(author.Handle),
		m.loadAuthorFeed(author.Handle, profileTabPosts, ""),
		m.loadAuthorFeed(author.Handle, profileTabReplies, ""),
	)
}

// syncDetailItemToFeed writes the like/repost state edited in the detail view
// back into every list that shows the same post.
func (m *Model) syncDetailItemToFeed() {
	for t := range m.feeds {
		for i, item := range m.feeds[t].items {
			if item.Post.URI == m.detailItem.Post.URI {
				m.feeds[t].items[i] = m.detailItem
			}
		}
	}
}

func (m *Model) isBookmarked(uri string) bool {
	for _, item := range m.feeds[tabSaved].items {
		if item.Post.URI == uri {
			return true
		}
	}
	return false
}
