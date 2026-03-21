package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
)

type fetchedMsg struct {
	tab    tab
	items  []api.FeedItem
	cursor string
	err    error
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
		client:      client,
		compose:     ta,
		searchInput: si,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		fetchFeed(m.client, tabHome),
		fetchFeed(m.client, tabDiscover),
		loadBookmarks(m.client),
	)
}

func fetchFeed(client *api.Client, t tab) tea.Cmd {
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

func loadMoreFeed(client *api.Client, t tab, cursor string) tea.Cmd {
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

func sendPost(client *api.Client, text string, replyTo *api.Post) tea.Cmd {
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

func likePost(client *api.Client, uri, cid string) tea.Cmd {
	return func() tea.Msg {
		likeURI, err := client.Like(uri, cid)
		return likeMsg{err: err, likeURI: likeURI, liked: true}
	}
}

func unlikePost(client *api.Client, likeURI string) tea.Cmd {
	return func() tea.Msg {
		err := client.Unlike(likeURI)
		return likeMsg{err: err, liked: false}
	}
}

func repostPost(client *api.Client, uri, cid string) tea.Cmd {
	return func() tea.Msg {
		repostURI, err := client.Repost(uri, cid)
		return repostMsg{err: err, repostURI: repostURI, reposted: true}
	}
}

func unrepostPost(client *api.Client, repostURI string) tea.Cmd {
	return func() tea.Msg {
		err := client.Unrepost(repostURI)
		return repostMsg{err: err, reposted: false}
	}
}

func bookmarkPost(client *api.Client, item api.FeedItem) tea.Cmd {
	return func() tea.Msg {
		err := client.CreateBookmark(item.Post.URI, item.Post.CID)
		return bookmarkMsg{err: err, bookmarked: true}
	}
}

func unbookmarkPost(client *api.Client, postURI string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteBookmark(postURI)
		return bookmarkMsg{err: err, bookmarked: false}
	}
}

func searchPosts(client *api.Client, query string) tea.Cmd {
	return func() tea.Msg {
		items, cursor, err := client.SearchPosts(query, 25, "")
		return searchMsg{items: items, cursor: cursor, err: err}
	}
}

func loadMoreSearch(client *api.Client, query, cursor string) tea.Cmd {
	return func() tea.Msg {
		items, nextCursor, err := client.SearchPosts(query, 25, cursor)
		return appendSearchMsg{items: items, cursor: nextCursor, err: err}
	}
}

func loadBookmarks(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		items, cursor, err := client.GetBookmarks(50, "")
		if err != nil {
			return fetchedMsg{tab: tabSaved, err: err}
		}
		return fetchedMsg{tab: tabSaved, items: items, cursor: cursor}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.compose.SetWidth(m.width/2 - 8)
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
			fetchFeed(m.client, tabHome),
			fetchFeed(m.client, tabDiscover),
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
			return m, loadBookmarks(m.client)
		} else {
			m.statusMsg = "Unbookmarked!"
			return m, loadBookmarks(m.client)
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
	}

	switch m.state {
	case stateCompose:
		return m.updateCompose(msg)
	case stateDetail:
		return m.updateDetail(msg)
	case stateSearch:
		return m.updateSearch(msg)
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
					return m, loadMoreSearch(m.client, m.searchQuery, m.searchNextCursor)
				}
			} else {
				feed := m.feeds[m.activeTab]
				if m.cursor[m.activeTab] < len(feed)-1 {
					m.cursor[m.activeTab]++
				} else if !m.loadingMore[m.activeTab] && m.nextCursor[m.activeTab] != "" && m.activeTab != tabSaved {
					m.loadingMore[m.activeTab] = true
					return m, loadMoreFeed(m.client, m.activeTab, m.nextCursor[m.activeTab])
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
				}
			} else {
				feed := m.feeds[m.activeTab]
				if len(feed) > 0 {
					m.detailItem = feed[m.cursor[m.activeTab]]
					m.state = stateDetail
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
					return m, loadBookmarks(m.client)
				}
				return m, fetchFeed(m.client, m.activeTab)
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
		}
	}
	return m, nil
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
		case "esc", "q":
			m.state = stateTimeline
			m.statusMsg = ""

		case "l":
			post := m.detailItem.Post
			if post.Viewer.Like != "" {
				return m, unlikePost(m.client, post.Viewer.Like)
			}
			return m, likePost(m.client, post.URI, post.CID)

		case "r":
			post := m.detailItem.Post
			if post.Viewer.Repost != "" {
				return m, unrepostPost(m.client, post.Viewer.Repost)
			}
			return m, repostPost(m.client, post.URI, post.CID)

		case "b":
			post := m.detailItem.Post
			if m.isBookmarked(post.URI) {
				return m, unbookmarkPost(m.client, post.URI)
			}
			return m, bookmarkPost(m.client, m.detailItem)

		case "c":
			p := m.detailItem.Post
			m.replyTo = &p
			m.prevState = stateDetail
			m.state = stateCompose
			m.composeErr = ""
			m.compose.Reset()
			m.compose.Focus()
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
			return m, sendPost(m.client, text, m.replyTo)
		}
	}
	m.compose, cmd = m.compose.Update(msg)
	return m, cmd
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
			return m, searchPosts(m.client, query)
		}
	}
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.state == stateDetail || (m.state == stateCompose && m.prevState == stateDetail) {
		base := m.renderDetailFull()
		if m.state == stateCompose {
			return m.renderOverlay(base)
		}
		return base
	}

	header := m.renderTabs()
	footer := m.renderStatusBar()
	help := m.renderHelpBar()
	contentHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - lipgloss.Height(help)

	timeline := m.renderTimeline(contentHeight)

	base := lipgloss.JoinVertical(lipgloss.Left, header, timeline, help, footer)

	if m.state == stateCompose {
		return m.renderOverlay(base)
	}
	if m.state == stateSearch {
		return m.renderSearchOverlay(base)
	}
	return base
}

func (m *Model) renderTabs() string {
	tabs := []string{"Home", "Discover", "Saved"}
	var rendered []string
	for i, name := range tabs {
		if tab(i) == m.activeTab {
			rendered = append(rendered, activeTabStyle.Render(name))
		} else {
			rendered = append(rendered, tabStyle.Render(name))
		}
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	divider := lipgloss.NewStyle().
		Foreground(colorBorder).
		Render(strings.Repeat("─", m.width))
	return lipgloss.JoinVertical(lipgloss.Left, line, divider)
}

func (m *Model) renderSearchResults(height int) string {
	if m.searchLoading {
		return lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(colorMuted).
			Render("Searching...")
	}

	feed := m.searchResults
	if len(feed) == 0 {
		return lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(colorMuted).
			Render("No results found.")
	}

	cur := m.searchCursor
	visibleLines := height - 2
	if visibleLines < 1 {
		visibleLines = 1
	}
	postsPerPage := visibleLines / 5
	if postsPerPage < 1 {
		postsPerPage = 1
	}
	start := 0
	if cur >= postsPerPage {
		start = cur - postsPerPage/2
	}
	end := start + postsPerPage + 1
	if end > len(feed) {
		end = len(feed)
	}

	var lines []string
	for i := start; i < end; i++ {
		post := feed[i].Post
		selected := i == cur

		name := post.Author.DisplayName
		if name == "" {
			name = post.Author.Handle
		}
		header := authorStyle.Render(name) + " " + handleStyle.Render("@"+post.Author.Handle)
		body := wrapText(post.Record.Text, m.width-8)
		stats := statsStyle.Render(fmt.Sprintf("♥ %d  ↺ %d  ✦ %d",
			post.LikeCount, post.RepostCount, post.ReplyCount))

		content := lipgloss.JoinVertical(lipgloss.Left,
			header,
			textStyle.Render(body),
			stats,
		)

		var rendered string
		if selected {
			rendered = selectedPostStyle.Width(m.width - 4).Render(content)
		} else {
			rendered = postStyle.Width(m.width - 4).Render(content)
		}
		lines = append(lines, rendered)
	}
	result := strings.Join(lines, "\n")
	if m.searchLoadingMore {
		result += "\n" + lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 2).Render("Loading more...")
	}
	return result
}

func (m *Model) renderTimeline(height int) string {
	if m.searchLoading || m.inSearch {
		return m.renderSearchResults(height)
	}

	t := m.activeTab
	if m.loading[t] {
		return lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(colorMuted).
			Render("Loading...")
	}
	if m.fetchErr[t] != "" {
		return errorStyle.Padding(1, 2).Render("Error: " + m.fetchErr[t])
	}

	feed := m.feeds[t]
	if len(feed) == 0 {
		return lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(colorMuted).
			Render("No posts yet.")
	}

	cur := m.cursor[t]
	// Calculate visible range
	visibleLines := height - 2
	if visibleLines < 1 {
		visibleLines = 1
	}

	// Estimate lines per post (~4 lines each)
	postsPerPage := visibleLines / 5
	if postsPerPage < 1 {
		postsPerPage = 1
	}

	start := 0
	if cur >= postsPerPage {
		start = cur - postsPerPage/2
	}
	end := start + postsPerPage + 1
	if end > len(feed) {
		end = len(feed)
	}

	var lines []string
	for i := start; i < end; i++ {
		post := feed[i].Post
		selected := i == cur

		name := post.Author.DisplayName
		if name == "" {
			name = post.Author.Handle
		}
		header := authorStyle.Render(name) + " " + handleStyle.Render("@"+post.Author.Handle)
		body := wrapText(post.Record.Text, m.width-8)
		stats := statsStyle.Render(fmt.Sprintf("♥ %d  ↺ %d  ✦ %d",
			post.LikeCount, post.RepostCount, post.ReplyCount))

		content := lipgloss.JoinVertical(lipgloss.Left,
			header,
			textStyle.Render(body),
			stats,
		)

		var rendered string
		if selected {
			rendered = selectedPostStyle.Width(m.width - 4).Render(content)
		} else {
			rendered = postStyle.Width(m.width - 4).Render(content)
		}
		lines = append(lines, rendered)
	}

	result := strings.Join(lines, "\n")
	if m.loadingMore[t] {
		loadingLine := lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 2).
			Render("Loading more...")
		result += "\n" + loadingLine
	}
	return result
}

func (m *Model) renderDetailFull() string {
	post := m.detailItem.Post

	name := post.Author.DisplayName
	if name == "" {
		name = post.Author.Handle
	}

	header := authorStyle.Render(name) + " " + handleStyle.Render("@"+post.Author.Handle)
	body := textStyle.Render(wrapText(post.Record.Text, m.width-8))
	bookmarkMark := ""
	if m.isBookmarked(post.URI) {
		bookmarkMark = "  ★"
	}
	stats := statsStyle.Render(fmt.Sprintf("♥ %d  ↺ %d  ✦ %d%s",
		post.LikeCount, post.RepostCount, post.ReplyCount, bookmarkMark))

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
		"",
		stats,
	)

	postBox := selectedPostStyle.Width(m.width - 4).Render(content)

	var statusLine string
	if m.statusMsg != "" {
		statusLine = successStyle.Render("  " + m.statusMsg)
	}

	divider := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", m.width))
	help := handleStyle.Width(m.width).Render("l: like/unlike  r: repost/unrepost  b: bookmark/unbookmark  c: comment  esc/q: back")
	footer := statusBarStyle.Width(m.width).Render("")

	main := lipgloss.JoinVertical(lipgloss.Left, divider, postBox, statusLine)
	return lipgloss.JoinVertical(lipgloss.Left, main, help, footer)
}

func (m *Model) renderHelpBar() string {
	var keys string
	if m.inSearch {
		keys = "j/k: scroll  enter: detail  s: new search  esc: clear search  q: quit"
	} else {
		keys = "j/k: scroll  h/l: tab  enter: detail  c: post  s: search  r: refresh  q: quit"
	}
	return handleStyle.Width(m.width).Render(keys)
}

func (m *Model) renderSearchOverlay(base string) string {
	overlayW := m.width / 2
	if overlayW < 50 {
		overlayW = 50
	}

	help := handleStyle.Render("Enter: search  Esc: cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		composeTitleStyle.Render("Search Posts"),
		m.searchInput.View(),
		help,
	)

	overlay := overlayStyle.Width(overlayW).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
	)
}

func (m *Model) renderStatusBar() string {
	var msg string
	if m.statusMsg != "" {
		if m.postSuccess {
			msg = successStyle.Render(m.statusMsg)
		} else {
			msg = m.statusMsg
		}
	}
	return statusBarStyle.Width(m.width).Render(msg)
}

func (m *Model) renderOverlay(base string) string {
	overlayW := m.width/2 + 4
	if overlayW < 50 {
		overlayW = 50
	}

	charCount := len([]rune(m.compose.Value()))
	remaining := 300 - charCount
	countColor := colorSubtext
	if remaining < 20 {
		countColor = colorError
	}

	countStr := lipgloss.NewStyle().Foreground(countColor).
		Render(fmt.Sprintf("%d/300", charCount))

	var errLine string
	if m.composeErr != "" {
		errLine = "\n" + errorStyle.Render(m.composeErr)
	}

	help := handleStyle.Render("Ctrl+S: post  Esc: cancel")

	title := "New Post"
	if m.replyTo != nil {
		replyName := m.replyTo.Author.DisplayName
		if replyName == "" {
			replyName = m.replyTo.Author.Handle
		}
		title = "Reply to " + replyName
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		composeTitleStyle.Render(title),
		m.compose.View(),
		lipgloss.JoinHorizontal(lipgloss.Top, countStr,
			lipgloss.NewStyle().Render(strings.Repeat(" ", overlayW-20-lipgloss.Width(countStr))),
			help),
		errLine,
	)

	overlay := overlayStyle.Width(overlayW).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
	)
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

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	words := strings.Fields(text)
	var lines []string
	var line strings.Builder
	lineLen := 0
	for _, w := range words {
		wl := len([]rune(w))
		if lineLen+wl+1 > width && lineLen > 0 {
			lines = append(lines, line.String())
			line.Reset()
			lineLen = 0
		}
		if lineLen > 0 {
			line.WriteString(" ")
			lineLen++
		}
		line.WriteString(w)
		lineLen += wl
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}
