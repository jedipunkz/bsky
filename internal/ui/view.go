package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jedipunkz/bsky/internal/api"
)

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

	if m.state == stateUserProfile && m.profilePrevState == stateDetail {
		base := m.renderDetailFull()
		return m.renderUserProfile(base)
	}

	header := m.renderTabs()
	footer := m.renderStatusBar()
	help := m.renderHelpBar()
	contentHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - lipgloss.Height(help)

	timeline := lipgloss.NewStyle().Height(contentHeight).Render(m.renderTimeline(contentHeight))

	base := lipgloss.JoinVertical(lipgloss.Left, header, timeline, help, footer)

	if m.state == stateCompose {
		return m.renderOverlay(base)
	}
	if m.state == stateSearch {
		return m.renderSearchOverlay(base)
	}
	if m.state == stateUserProfile {
		return m.renderUserProfile(base)
	}
	return base
}

func (m *Model) renderTabs() string {
	var line string
	if m.search.loading || m.inSearch {
		line = activeTabStyle.Render("Search Result")
	} else {
		tabs := []string{"Home", "Discover", "Saved"}
		var rendered []string
		for i, name := range tabs {
			if tab(i) == m.activeTab {
				rendered = append(rendered, activeTabStyle.Render(name))
			} else {
				rendered = append(rendered, tabStyle.Render(name))
			}
		}
		line = lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	}
	if m.client != nil && m.client.Handle != "" {
		account := handleStyle.Render("@" + m.client.Handle + " ")
		if pad := m.width - lipgloss.Width(line) - lipgloss.Width(account); pad > 0 {
			line += strings.Repeat(" ", pad) + account
		}
	}
	divider := dividerStyle.Render(strings.Repeat("─", m.width))
	return lipgloss.JoinVertical(lipgloss.Left, line, divider)
}

// authorName returns the display name, falling back to the handle.
func authorName(a api.Author) string {
	if a.DisplayName != "" {
		return a.DisplayName
	}
	return a.Handle
}

// relativeTime renders an RFC3339 timestamp as a compact age ("3m", "2h", "5d").
// Returns "" when the timestamp is missing or unparsable.
func relativeTime(createdAt string) string {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	return t.Format("Jan 2")
}

// renderPostHeader renders "name @handle" on the left and the post age on the right.
func renderPostHeader(post api.Post, width int, nameSt, handleSt lipgloss.Style) string {
	left := nameSt.Render(ansi.Truncate(authorName(post.Author), 24, "…")) +
		" " + handleSt.Render("@"+ansi.Truncate(post.Author.Handle, 30, "…"))
	age := relativeTime(post.Record.CreatedAt)
	if age == "" {
		return left
	}
	right := timeStyle.Render(age)
	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		return left
	}
	return left + strings.Repeat(" ", pad) + right
}

// renderPostMeta renders the counters line, omitting anything that is zero so
// that quiet posts stay visually quiet. Returns "" when there is nothing to show.
func (m *Model) renderPostMeta(post api.Post) string {
	var parts []string
	if post.Record.Reply != nil {
		parts = append(parts, metaStyle.Render("↩ reply"))
	}
	if post.LikeCount > 0 || post.Viewer.Like != "" {
		st := metaStyle
		if post.Viewer.Like != "" {
			st = likedStyle
		}
		parts = append(parts, st.Render(fmt.Sprintf("♥ %d", post.LikeCount)))
	}
	if post.RepostCount > 0 || post.Viewer.Repost != "" {
		st := metaStyle
		if post.Viewer.Repost != "" {
			st = repostedStyle
		}
		parts = append(parts, st.Render(fmt.Sprintf("↺ %d", post.RepostCount)))
	}
	if post.ReplyCount > 0 {
		parts = append(parts, metaStyle.Render(fmt.Sprintf("✦ %d", post.ReplyCount)))
	}
	if n := len(post.Embed.EmbedImages()); n > 0 {
		parts = append(parts, metaStyle.Render(fmt.Sprintf("🖼 %d", n)))
	}
	if m.isBookmarked(post.URI) {
		parts = append(parts, bookmarkedStyle.Render("★"))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, metaStyle.Render("  "))
}

// renderFeedItem renders a single feed item as a styled post box of the given
// outer width (border and padding included).
func (m *Model) renderFeedItem(item api.FeedItem, selected bool, width int) string {
	nameSt, handleSt, textSt := authorStyle, handleStyle, textStyle
	boxSt := postStyle
	if selected {
		nameSt, handleSt, textSt = selectedAuthorStyle, selectedHandleStyle, selectedTextStyle
		boxSt = selectedPostStyle
	}

	inner := width - 2 // horizontal padding (2); the left border sits outside Width
	if inner < 20 {
		inner = 20
	}

	post := item.Post
	parts := []string{
		renderPostHeader(post, inner, nameSt, handleSt),
		renderTextWithURLsStyled(post.Record.Text, inner, textSt),
	}
	if meta := m.renderPostMeta(post); meta != "" {
		parts = append(parts, meta)
	}

	return boxSt.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

// fillFeedToHeight renders feed items around cur, expanding to fill height lines.
// Posts that don't fully fit are truncated so the terminal is always filled.
func (m *Model) fillFeedToHeight(l *feedList, height int) []string {
	feed, cur := l.items, l.cursor
	if len(feed) == 0 {
		return nil
	}

	truncateToLines := func(s string, n int) string {
		if n <= 0 {
			return ""
		}
		ls := strings.Split(s, "\n")
		if len(ls) <= n {
			return s
		}
		return strings.Join(ls[:n], "\n")
	}

	// strings.Join(lines, "\n") does NOT add extra lines:
	// height(join([r1,r2],"\n")) == height(r1) + height(r2).
	// So usedHeight is simply the sum of individual post heights.

	var forward []string
	usedHeight := 0
	for i := cur; i < len(feed); i++ {
		r := m.renderFeedItem(feed[i], i == cur, m.width-4)
		h := lipgloss.Height(r)
		remaining := height - usedHeight
		if remaining <= 0 {
			break
		}
		if h > remaining {
			// Truncate to fill the rest of the screen, then stop
			forward = append(forward, truncateToLines(r, remaining))
			usedHeight += remaining
			break
		}
		usedHeight += h
		forward = append(forward, r)
	}

	// Fill remaining space by expanding backward from cursor-1
	var backward []string
	for i := cur - 1; i >= 0; i-- {
		r := m.renderFeedItem(feed[i], false, m.width-4)
		h := lipgloss.Height(r)
		if usedHeight+h > height {
			break
		}
		usedHeight += h
		backward = append(backward, r)
	}

	// Combine: backward entries are in reverse order, prepend them
	lines := make([]string, 0, len(backward)+len(forward))
	for j := len(backward) - 1; j >= 0; j-- {
		lines = append(lines, backward[j])
	}
	lines = append(lines, forward...)
	return lines
}

// renderList renders one feed list into exactly height rows, including its
// loading, error and empty states.
func (m *Model) renderList(l *feedList, height int, loadingMsg, emptyMsg string) string {
	notice := func(text string) string {
		return lipgloss.NewStyle().Padding(1, 2).Foreground(colorMuted).Render(text)
	}

	switch {
	case l.loading:
		return notice(loadingMsg)
	case l.err != "":
		return errorStyle.Padding(1, 2).Render("Error: " + l.err)
	case len(l.items) == 0:
		return notice(emptyMsg)
	}

	result := strings.Join(m.fillFeedToHeight(l, height), "\n")
	if l.loadingMore {
		result += "\n" + lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 2).Render("Loading more...")
	}
	return result
}

func (m *Model) renderTimeline(height int) string {
	if m.search.loading || m.inSearch {
		return m.renderList(&m.search, height, "Searching...", "No results found.")
	}
	return m.renderList(&m.feeds[m.activeTab], height, "Loading...", "No posts yet.")
}

func (m *Model) renderDetailFull() string {
	post := m.detailItem.Post

	header := renderPostHeader(post, m.width-6, selectedAuthorStyle, selectedHandleStyle)
	body := renderTextWithURLsStyled(post.Record.Text, m.width-6, selectedTextStyle)

	// Stats and help are rendered outside the postBox so the image can sit between them.
	stats := lipgloss.NewStyle().Padding(0, 1).Render(m.renderPostMeta(post))

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
	)

	postBox := selectedPostStyle.Width(m.width - 4).Render(content)

	var statusLine string
	if m.statusMsg != "" {
		statusLine = successStyle.Render("  " + m.statusMsg)
	}

	divider := dividerStyle.Render(strings.Repeat("─", m.width))
	help := lipgloss.NewStyle().Width(m.width).Padding(0, 1).Render(
		keyHints("l", "like", "r", "repost", "b", "bookmark", "c", "comment", "u", "profile", "⏎/esc", "back"))
	footer := statusBarStyle.Width(m.width).Render("")

	// Build the top section (everything above the image).
	topParts := []string{divider, postBox}
	if statusLine != "" {
		topParts = append(topParts, statusLine)
	}
	top := lipgloss.JoinVertical(lipgloss.Left, topParts...)

	// Fixed rows at the bottom: stats (1) + help (1) + footer (1).
	const fixedBottomRows = 3
	availableForImage := m.height - lipgloss.Height(top) - fixedBottomRows
	if availableForImage < 0 {
		availableForImage = 0
	}

	// Build image section if an embed image exists, padded to exactly availableForImage rows
	// so that stats/help/footer always appear at a stable position below the image area.
	maxCols := m.width - 4
	if maxCols < 20 {
		maxCols = 20
	}

	// Build image block: always exactly availableForImage rows (availableForImage-1 \n chars).
	var imgBlock string
	embedImgs := post.Embed.EmbedImages()
	if len(embedImgs) > 0 && availableForImage > 0 {
		imgURL := embedImgs[0].Fullsize
		if imgURL == "" {
			imgURL = embedImgs[0].Thumb
		}
		if img, ok := m.imageCache[imgURL]; ok {
			// Render at display time so size always matches current available space.
			imgBlock = renderImageForView(img, maxCols, availableForImage)
		} else if errMsg, hasErr := m.imageError[imgURL]; hasErr {
			imgBlock = errorStyle.Padding(0, 2).Render("🖼 image load error: " + errMsg)
			imgBlock += strings.Repeat("\n", availableForImage-1)
		} else {
			imgBlock = lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 2).Render("🖼 loading...")
			imgBlock += strings.Repeat("\n", availableForImage-1)
		}
	} else if availableForImage > 0 {
		imgBlock = strings.Repeat("\n", availableForImage-1)
	}

	result := top
	if imgBlock != "" {
		result += "\n" + imgBlock
	}
	result += "\n" + stats + "\n" + help + "\n" + footer
	return result
}

// keyHints renders alternating key/description pairs as a dimmed hint line
// with the keys highlighted.
func keyHints(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, keyStyle.Render(pairs[i])+" "+keyDescStyle.Render(pairs[i+1]))
	}
	return strings.Join(parts, keyDescStyle.Render(" · "))
}

func (m *Model) renderHelpBar() string {
	var keys string
	if m.inSearch {
		keys = keyHints("j/k", "scroll", "⏎", "detail", "u", "profile", "s", "new search", "esc", "clear", "q", "quit")
	} else {
		keys = keyHints("j/k", "scroll", "h/l", "tab", "⏎", "detail", "u", "profile", "c", "post", "s", "search", "r", "refresh", "q", "quit")
	}
	return lipgloss.NewStyle().Width(m.width).Padding(0, 1).Render(keys)
}

func (m *Model) renderSearchOverlay(base string) string {
	overlayW := m.width / 2
	if overlayW < 50 {
		overlayW = 50
	}

	help := keyHints("⏎", "search", "esc", "cancel")

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

	help := keyHints("ctrl+s", "post", "esc", "cancel")

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

func (m *Model) renderUserProfile(base string) string {
	overlayW := m.width * 4 / 5
	if overlayW < 66 {
		overlayW = 66
	}
	overlayH := m.height * 4 / 5
	if overlayH < 20 {
		overlayH = 20
	}
	innerW := overlayW - 6 // border(2) + padding(4)

	// Profile header
	var headerParts []string
	if m.profileLoading {
		headerParts = append(headerParts, lipgloss.NewStyle().Foreground(colorMuted).Render("Loading profile..."))
	} else if m.profileData == nil {
		headerParts = append(headerParts, lipgloss.NewStyle().Foreground(colorError).Render("Failed to load profile"))
	} else {
		p := m.profileData
		name := p.DisplayName
		if name == "" {
			name = p.Handle
		}
		title := selectedAuthorStyle.Render(name) + " " + handleStyle.Render("@"+p.Handle)
		if p.Viewer.Following != "" {
			title += "  " + successStyle.Render("✓ following")
		}
		headerParts = append(headerParts, title)
		headerParts = append(headerParts, metaStyle.Render(fmt.Sprintf(
			"%d followers  ·  %d following  ·  %d posts",
			p.FollowersCount, p.FollowsCount, p.PostsCount,
		)))
	}
	header := strings.Join(headerParts, "\n")

	// Tab bar
	var tabPosts, tabReplies string
	if m.profileActiveTab == profileTabPosts {
		tabPosts = activeTabStyle.Render("Posts")
		tabReplies = tabStyle.Render("Replies")
	} else {
		tabPosts = tabStyle.Render("Posts")
		tabReplies = activeTabStyle.Render("Replies")
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabPosts, tabReplies)

	divider := dividerStyle.Render(strings.Repeat("─", innerW))
	help := keyHints("h/l", "tab", "j/k", "scroll", "f", "follow", "q", "back")

	// Measure non-posts content height accurately
	frameContent := lipgloss.JoinVertical(lipgloss.Left, header, "", tabBar, divider, help)
	// border(2) + padding top+bottom(2)
	postsHeight := overlayH - lipgloss.Height(frameContent) - 4
	if postsHeight < 3 {
		postsHeight = 3
	}

	postsContent := m.renderProfilePosts(innerW, postsHeight)

	// Clip to exact postsHeight lines so overlay size stays fixed
	postsLines := strings.Split(postsContent, "\n")
	if len(postsLines) > postsHeight {
		postsLines = postsLines[:postsHeight]
	}
	for len(postsLines) < postsHeight {
		postsLines = append(postsLines, "")
	}
	postsContent = strings.Join(postsLines, "\n")

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		tabBar,
		divider,
		postsContent,
		help,
	)

	overlay := overlayStyle.Width(innerW).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
	)
}

func (m *Model) renderProfilePosts(width, height int) string {
	l := &m.profileFeeds[m.profileActiveTab]
	switch {
	case l.loading:
		return lipgloss.NewStyle().Foreground(colorMuted).Render("Loading posts...")
	case len(l.items) == 0:
		return lipgloss.NewStyle().Foreground(colorMuted).Render("No posts.")
	}

	const linesPerPost = 4 // author + text + meta + spacing
	visiblePosts := height / linesPerPost
	if visiblePosts < 1 {
		visiblePosts = 1
	}

	start := 0
	if l.cursor >= visiblePosts {
		start = l.cursor - visiblePosts/2
	}
	end := min(start+visiblePosts+1, len(l.items))

	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, m.renderFeedItem(l.items[i], i == l.cursor, width-4))
	}
	result := strings.Join(lines, "\n")
	if l.loadingMore {
		result += "\n" + lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 2).Render("Loading more...")
	}
	return result
}

// renderTextWithURLsStyled wraps text and renders URLs as OSC 8 terminal
// hyperlinks (underlined, primary color). Shift+click opens the URL in the browser.
func renderTextWithURLsStyled(text string, width int, ts lipgloss.Style) string {
	wrapped := wrapText(text, width)
	matches := urlRegex.FindAllStringIndex(wrapped, -1)
	if len(matches) == 0 {
		return renderLines(ts, wrapped)
	}

	var b strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if start > last {
			b.WriteString(renderLines(ts, wrapped[last:start]))
		}
		rawURL := wrapped[start:end]
		styled := linkStyle.Render(rawURL)
		// OSC 8 hyperlink: \033]8;;URL\a + visible text + \033]8;;\a
		b.WriteString("\033]8;;" + rawURL + "\a" + styled + "\033]8;;\a")
		last = end
	}
	if last < len(wrapped) {
		b.WriteString(renderLines(ts, wrapped[last:]))
	}
	return b.String()
}

// renderLines styles each line individually. Rendering a multi-line string in
// one call would make lipgloss pad every line to the block width, which
// misplaces text that follows on the same line (e.g. a URL after a wrap).
func renderLines(st lipgloss.Style, s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = st.Render(l)
	}
	return strings.Join(lines, "\n")
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	// ansi.Wrap is display-width aware, so CJK (2-cell) runes and words without
	// spaces wrap correctly instead of overflowing the post box.
	return ansi.Wrap(text, width, "")
}

var urlRegex = regexp.MustCompile(`https?://[^\s]+`)
