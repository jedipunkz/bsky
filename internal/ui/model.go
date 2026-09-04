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

type fetchedMsg struct {
	tab    tab
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

type appendedMsg struct {
	tab    tab
	items  []api.FeedItem
	cursor string
	err    error
}

type searchMsg struct {
	items  []api.FeedItem
	cursor string
	err    error
}

type appendSearchMsg struct {
	items  []api.FeedItem
	cursor string
	err    error
}

type fetchedProfileMsg struct {
	profile *api.Profile
	err     error
}

type followMsg struct {
	err       error
	followURI string
	followed  bool
}

type fetchedAuthorFeedMsg struct {
	tabType profileTabType
	items   []api.FeedItem
	cursor  string
	err     error
}

type appendedAuthorFeedMsg struct {
	tabType profileTabType
	items   []api.FeedItem
	cursor  string
	err     error
}

type Model struct {
	client    *api.Client
	width     int
	height    int
	activeTab tab
	state     state
	prevState state

	feeds       [tabCount][]api.FeedItem
	cursor      [tabCount]int
	loading     [tabCount]bool
	loadingMore [tabCount]bool
	nextCursor  [tabCount]string
	fetchErr    [tabCount]string

	detailItem api.FeedItem
	replyTo    *api.Post

	compose     textarea.Model
	composeErr  string
	postSuccess bool

	searchInput       textinput.Model
	searchResults     []api.FeedItem
	searchCursor      int
	searchLoading     bool
	searchLoadingMore bool
	searchNextCursor  string
	inSearch          bool
	searchQuery       string

	statusMsg string

	profileActor           string
	profileData            *api.Profile
	profileActiveTab       profileTabType
	profileFeeds           [profileTabCount][]api.FeedItem
	profileCursors         [profileTabCount]int
	profileLoading         bool
	profileFeedLoading     [profileTabCount]bool
	profileFeedLoadingMore [profileTabCount]bool
	profileNextCursors     [profileTabCount]string
	profilePrevState       state

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
		m.fetchFeed(tabHome),
		m.fetchFeed(tabDiscover),
		m.loadBookmarks(),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.compose.SetWidth(m.width/2 - 8)
		return m, nil

	case imageFetchedMsg:
		delete(m.imageLoading, msg.url)
		if msg.err != nil {
			m.imageError[msg.url] = msg.err.Error()
		} else {
			m.imageCache[msg.url] = msg.img
		}
		return m, nil

	case fetchedMsg:
		m.loading[msg.tab] = false
		if msg.err != nil {
			m.fetchErr[msg.tab] = msg.err.Error()
		} else {
			m.feeds[msg.tab] = msg.items
			m.nextCursor[msg.tab] = msg.cursor
			m.fetchErr[msg.tab] = ""
		}
		return m, nil

	case appendedMsg:
		m.loadingMore[msg.tab] = false
		if msg.err != nil {
			m.fetchErr[msg.tab] = msg.err.Error()
		} else {
			m.feeds[msg.tab] = append(m.feeds[msg.tab], msg.items...)
			m.nextCursor[msg.tab] = msg.cursor
			m.fetchErr[msg.tab] = ""
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
			m.fetchFeed(tabHome),
			m.fetchFeed(tabDiscover),
		)

	case likeMsg:
		if msg.err != nil {
			m.statusMsg = "Like failed: " + msg.err.Error()
		} else if msg.liked {
			m.statusMsg = "Liked!"
			m.detailItem.Post.LikeCount++
			m.detailItem.Post.Viewer.Like = msg.likeURI
			m.syncDetailItemToFeed()
		} else {
			m.statusMsg = "Unliked!"
			if m.detailItem.Post.LikeCount > 0 {
				m.detailItem.Post.LikeCount--
			}
			m.detailItem.Post.Viewer.Like = ""
			m.syncDetailItemToFeed()
		}
		return m, nil

	case repostMsg:
		if msg.err != nil {
			m.statusMsg = "Repost failed: " + msg.err.Error()
		} else if msg.reposted {
			m.statusMsg = "Reposted!"
			m.detailItem.Post.RepostCount++
			m.detailItem.Post.Viewer.Repost = msg.repostURI
			m.syncDetailItemToFeed()
		} else {
			m.statusMsg = "Unreposted!"
			if m.detailItem.Post.RepostCount > 0 {
				m.detailItem.Post.RepostCount--
			}
			m.detailItem.Post.Viewer.Repost = ""
			m.syncDetailItemToFeed()
		}
		return m, nil

	case bookmarkMsg:
		if msg.err != nil {
			m.statusMsg = "Bookmark failed: " + msg.err.Error()
		} else if msg.bookmarked {
			m.statusMsg = "Bookmarked!"
			return m, m.loadBookmarks()
		} else {
			m.statusMsg = "Unbookmarked!"
			return m, m.loadBookmarks()
		}
		return m, nil

	case searchMsg:
		m.searchLoading = false
		if msg.err != nil {
			m.statusMsg = "Search failed: " + msg.err.Error()
			m.inSearch = false
		} else {
			filtered := filterSearchResults(msg.items, m.searchQuery)
			m.searchResults = filtered
			m.searchNextCursor = msg.cursor
			m.searchCursor = 0
			m.inSearch = true
			m.statusMsg = fmt.Sprintf("Search: %q (%d results)", m.searchQuery, len(filtered))
		}
		return m, nil

	case appendSearchMsg:
		m.searchLoadingMore = false
		if msg.err == nil {
			filtered := filterSearchResults(msg.items, m.searchQuery)
			m.searchResults = append(m.searchResults, filtered...)
			m.searchNextCursor = msg.cursor
			m.statusMsg = fmt.Sprintf("Search: %q (%d results)", m.searchQuery, len(m.searchResults))
		}
		return m, nil

	case followMsg:
		if msg.err != nil {
			m.statusMsg = "Follow failed: " + msg.err.Error()
		} else if msg.followed {
			m.statusMsg = "Followed!"
			if m.profileData != nil {
				m.profileData.Viewer.Following = msg.followURI
				m.profileData.FollowersCount++
			}
		} else {
			m.statusMsg = "Unfollowed!"
			if m.profileData != nil {
				m.profileData.Viewer.Following = ""
				if m.profileData.FollowersCount > 0 {
					m.profileData.FollowersCount--
				}
			}
		}
		return m, nil

	case fetchedProfileMsg:
		m.profileLoading = false
		if msg.err == nil {
			m.profileData = msg.profile
		}
		return m, nil

	case fetchedAuthorFeedMsg:
		m.profileFeedLoading[msg.tabType] = false
		if msg.err == nil {
			m.profileFeeds[msg.tabType] = msg.items
			m.profileNextCursors[msg.tabType] = msg.cursor
		}
		return m, nil

	case appendedAuthorFeedMsg:
		m.profileFeedLoadingMore[msg.tabType] = false
		if msg.err == nil {
			m.profileFeeds[msg.tabType] = append(m.profileFeeds[msg.tabType], msg.items...)
			m.profileNextCursors[msg.tabType] = msg.cursor
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

func (m *Model) updateTimeline(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.postSuccess = false
		switch msg.String() {
		case "q":
			if m.inSearch {
				m.inSearch = false
				m.searchResults = nil
				m.searchCursor = 0
				m.searchQuery = ""
				m.searchNextCursor = ""
				m.searchLoadingMore = false
				m.statusMsg = ""
			} else {
				return m, tea.Quit
			}

		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			if m.inSearch {
				m.inSearch = false
				m.searchResults = nil
				m.searchCursor = 0
				m.searchQuery = ""
				m.searchNextCursor = ""
				m.searchLoadingMore = false
				m.statusMsg = ""
			}

		case "j":
			m.statusMsg = ""
			if m.inSearch {
				if m.searchCursor < len(m.searchResults)-1 {
					m.searchCursor++
				} else if !m.searchLoadingMore && m.searchNextCursor != "" {
					m.searchLoadingMore = true
					return m, m.loadMoreSearch(m.searchQuery, m.searchNextCursor)
				}
			} else {
				feed := m.feeds[m.activeTab]
				if m.cursor[m.activeTab] < len(feed)-1 {
					m.cursor[m.activeTab]++
				} else if !m.loadingMore[m.activeTab] && m.nextCursor[m.activeTab] != "" && m.activeTab != tabSaved {
					m.loadingMore[m.activeTab] = true
					return m, m.loadMoreFeed(m.activeTab, m.nextCursor[m.activeTab])
				}
			}

		case "k":
			m.statusMsg = ""
			if m.inSearch {
				if m.searchCursor > 0 {
					m.searchCursor--
				}
			} else {
				if m.cursor[m.activeTab] > 0 {
					m.cursor[m.activeTab]--
				}
			}

		case "h":
			if !m.inSearch {
				m.statusMsg = ""
				if m.activeTab > 0 {
					m.activeTab--
				}
			}

		case "l":
			if !m.inSearch {
				m.statusMsg = ""
				if m.activeTab < tabCount-1 {
					m.activeTab++
				}
			}

		case "enter":
			m.statusMsg = ""
			if m.inSearch {
				if len(m.searchResults) > 0 {
					m.detailItem = m.searchResults[m.searchCursor]
					m.state = stateDetail
					return m, m.fetchDetailImageCmd()
				}
			} else {
				feed := m.feeds[m.activeTab]
				if len(feed) > 0 {
					m.detailItem = feed[m.cursor[m.activeTab]]
					m.state = stateDetail
					return m, m.fetchDetailImageCmd()
				}
			}

		case "c":
			m.statusMsg = ""
			m.prevState = stateTimeline
			m.state = stateCompose
			m.replyTo = nil
			m.composeErr = ""
			m.compose.Reset()
			m.compose.Focus()

		case "s":
			m.state = stateSearch
			m.searchInput.SetValue("")
			m.searchInput.Focus()

		case "r":
			if !m.inSearch {
				m.statusMsg = ""
				m.loading[m.activeTab] = true
				m.cursor[m.activeTab] = 0
				if m.activeTab == tabSaved {
					return m, m.loadBookmarks()
				}
				return m, m.fetchFeed(m.activeTab)
			}

		case "g":
			m.statusMsg = ""
			if m.inSearch {
				m.searchCursor = 0
			} else {
				m.cursor[m.activeTab] = 0
			}

		case "G":
			m.statusMsg = ""
			if m.inSearch {
				if len(m.searchResults) > 0 {
					m.searchCursor = len(m.searchResults) - 1
				}
			} else {
				feed := m.feeds[m.activeTab]
				if len(feed) > 0 {
					m.cursor[m.activeTab] = len(feed) - 1
				}
			}

		case "u":
			m.statusMsg = ""
			var author api.Author
			if m.inSearch {
				if len(m.searchResults) > 0 {
					author = m.searchResults[m.searchCursor].Post.Author
				}
			} else {
				feed := m.feeds[m.activeTab]
				if len(feed) > 0 {
					author = feed[m.cursor[m.activeTab]].Post.Author
				}
			}
			if author.DID != "" {
				return m.openUserProfile(author, stateTimeline)
			}
		}
	}
	return m, nil
}

func (m *Model) openUserProfile(author api.Author, prevState state) (tea.Model, tea.Cmd) {
	m.profileActor = author.Handle
	m.profileData = nil
	m.profileFeeds = [profileTabCount][]api.FeedItem{}
	m.profileCursors = [profileTabCount]int{}
	m.profileNextCursors = [profileTabCount]string{}
	m.profileFeedLoadingMore = [profileTabCount]bool{}
	m.profileActiveTab = profileTabPosts
	m.profileLoading = true
	m.profileFeedLoading = [profileTabCount]bool{true, true}
	m.profilePrevState = prevState
	m.state = stateUserProfile
	return m, tea.Batch(
		m.fetchProfile(author.Handle),
		m.fetchAuthorFeed(author.Handle, profileTabPosts),
		m.fetchAuthorFeed(author.Handle, profileTabReplies),
	)
}

func (m *Model) syncDetailItemToFeed() {
	for t := tab(0); t < tabCount; t++ {
		for i, item := range m.feeds[t] {
			if item.Post.URI == m.detailItem.Post.URI {
				m.feeds[t][i] = m.detailItem
			}
		}
	}
}

func (m *Model) isBookmarked(uri string) bool {
	for _, item := range m.feeds[tabSaved] {
		if item.Post.URI == uri {
			return true
		}
	}
	return false
}

func (m *Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "enter":
			m.state = stateTimeline
			m.statusMsg = ""
			return m, tea.ClearScreen

		case "l":
			post := m.detailItem.Post
			if post.Viewer.Like != "" {
				return m, m.unlikePost(post.Viewer.Like)
			}
			return m, m.likePost(post.URI, post.CID)

		case "r":
			post := m.detailItem.Post
			if post.Viewer.Repost != "" {
				return m, m.unrepostPost(post.Viewer.Repost)
			}
			return m, m.repostPost(post.URI, post.CID)

		case "b":
			post := m.detailItem.Post
			if m.isBookmarked(post.URI) {
				return m, m.unbookmarkPost(post.URI)
			}
			return m, m.bookmarkPost(m.detailItem)

		case "c":
			p := m.detailItem.Post
			m.replyTo = &p
			m.prevState = stateDetail
			m.state = stateCompose
			m.composeErr = ""
			m.compose.Reset()
			m.compose.Focus()

		case "u":
			return m.openUserProfile(m.detailItem.Post.Author, stateDetail)
		}
	}
	return m, nil
}

func (m *Model) updateCompose(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.state = m.prevState
			m.replyTo = nil
			m.composeErr = ""
			return m, nil
		case "ctrl+s":
			text := strings.TrimSpace(m.compose.Value())
			if text == "" {
				m.composeErr = "Post cannot be empty"
				return m, nil
			}
			if len([]rune(text)) > 300 {
				m.composeErr = "Post exceeds 300 characters"
				return m, nil
			}
			return m, m.sendPost(text, m.replyTo)
		}
	}
	m.compose, cmd = m.compose.Update(msg)
	return m, cmd
}

func (m *Model) updateUserProfile(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.state = m.profilePrevState
			m.statusMsg = ""
		case "f":
			if m.profileData == nil {
				return m, nil
			}
			if m.profileData.Viewer.Following != "" {
				return m, m.unfollowUser(m.profileData.Viewer.Following)
			}
			return m, m.followUser(m.profileData.DID)
		case "h":
			if m.profileActiveTab > 0 {
				m.profileActiveTab--
			}
		case "l":
			if m.profileActiveTab < profileTabCount-1 {
				m.profileActiveTab++
			}
		case "j":
			t := m.profileActiveTab
			feed := m.profileFeeds[t]
			if m.profileCursors[t] < len(feed)-1 {
				m.profileCursors[t]++
			} else if !m.profileFeedLoadingMore[t] && m.profileNextCursors[t] != "" {
				m.profileFeedLoadingMore[t] = true
				return m, m.loadMoreAuthorFeed(m.profileActor, t, m.profileNextCursors[t])
			}
		case "k":
			if m.profileCursors[m.profileActiveTab] > 0 {
				m.profileCursors[m.profileActiveTab]--
			}
		case "g":
			m.profileCursors[m.profileActiveTab] = 0
		case "G":
			feed := m.profileFeeds[m.profileActiveTab]
			if len(feed) > 0 {
				m.profileCursors[m.profileActiveTab] = len(feed) - 1
			}
		}
	}
	return m, nil
}

func (m *Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.state = stateTimeline
			return m, nil
		case "enter":
			query := strings.TrimSpace(m.searchInput.Value())
			if query == "" {
				return m, nil
			}
			m.searchQuery = query
			m.searchLoading = true
			m.inSearch = false
			m.state = stateTimeline
			return m, m.searchPosts(query)
		}
	}
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}
