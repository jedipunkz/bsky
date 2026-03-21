package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedipunkz/bsky/internal/api"
)

type tab int

const (
	tabHome tab = iota
	tabDiscover
	tabCount
)

type state int

const (
	stateTimeline state = iota
	stateCompose
)

type fetchedMsg struct {
	tab   tab
	items []api.FeedItem
	err   error
}

type postSentMsg struct {
	err error
}

type Model struct {
	client    *api.Client
	width     int
	height    int
	activeTab tab
	state     state

	feeds    [tabCount][]api.FeedItem
	cursor   [tabCount]int
	loading  [tabCount]bool
	fetchErr [tabCount]string

	compose     textarea.Model
	composeErr  string
	postSuccess bool

	statusMsg string
}

func New(client *api.Client, theme string) *Model {
	applyTheme(theme)

	ta := textarea.New()
	ta.Placeholder = "What's on your mind? (Ctrl+Enter to post, Esc to cancel)"
	ta.CharLimit = 300
	ta.SetWidth(60)
	ta.SetHeight(5)
	ta.Focus()

	return &Model{
		client:  client,
		compose: ta,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		fetchFeed(m.client, tabHome),
		fetchFeed(m.client, tabDiscover),
	)
}

func fetchFeed(client *api.Client, t tab) tea.Cmd {
	return func() tea.Msg {
		var items []api.FeedItem
		var err error
		if t == tabHome {
			items, err = client.GetTimeline(50)
		} else {
			items, err = client.GetDiscoverFeed(50)
		}
		return fetchedMsg{tab: t, items: items, err: err}
	}
}

func sendPost(client *api.Client, text string) tea.Cmd {
	return func() tea.Msg {
		err := client.CreatePost(text)
		return postSentMsg{err: err}
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
			m.fetchErr[msg.tab] = ""
		}
		return m, nil

	case postSentMsg:
		if msg.err != nil {
			m.composeErr = msg.err.Error()
			return m, nil
		}
		m.state = stateTimeline
		m.compose.Reset()
		m.composeErr = ""
		m.postSuccess = true
		m.statusMsg = "Post sent!"
		return m, tea.Batch(
			fetchFeed(m.client, tabHome),
			fetchFeed(m.client, tabDiscover),
		)
	}

	if m.state == stateCompose {
		return m.updateCompose(msg)
	}
	return m.updateTimeline(msg)
}

func (m *Model) updateTimeline(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.statusMsg = ""
		m.postSuccess = false
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "j":
			feed := m.feeds[m.activeTab]
			if m.cursor[m.activeTab] < len(feed)-1 {
				m.cursor[m.activeTab]++
			}

		case "k":
			if m.cursor[m.activeTab] > 0 {
				m.cursor[m.activeTab]--
			}

		case "h":
			if m.activeTab > 0 {
				m.activeTab--
			}

		case "l":
			if m.activeTab < tabCount-1 {
				m.activeTab++
			}

		case "c":
			m.state = stateCompose
			m.composeErr = ""
			m.compose.Reset()
			m.compose.Focus()

		case "r":
			m.loading[m.activeTab] = true
			return m, fetchFeed(m.client, m.activeTab)

		case "g":
			m.cursor[m.activeTab] = 0

		case "G":
			feed := m.feeds[m.activeTab]
			if len(feed) > 0 {
				m.cursor[m.activeTab] = len(feed) - 1
			}
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
			m.state = stateTimeline
			m.composeErr = ""
			return m, nil
		case "ctrl+enter":
			text := strings.TrimSpace(m.compose.Value())
			if text == "" {
				m.composeErr = "Post cannot be empty"
				return m, nil
			}
			if len([]rune(text)) > 300 {
				m.composeErr = "Post exceeds 300 characters"
				return m, nil
			}
			return m, sendPost(m.client, text)
		}
	}
	m.compose, cmd = m.compose.Update(msg)
	return m, cmd
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
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
	return base
}

func (m *Model) renderTabs() string {
	tabs := []string{"Home", "Discover"}
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

func (m *Model) renderTimeline(height int) string {
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

	return strings.Join(lines, "\n")
}

func (m *Model) renderHelpBar() string {
	keys := "j/k: scroll  h/l: tab  c: post  r: refresh  q: quit"
	return handleStyle.Width(m.width).Render(keys)
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

	help := handleStyle.Render("Ctrl+Enter: post  Esc: cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		composeTitleStyle.Render("New Post"),
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
