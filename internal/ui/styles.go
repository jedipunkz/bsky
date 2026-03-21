package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary  = lipgloss.Color("#0085ff")
	colorMuted    = lipgloss.Color("#888888")
	colorBorder   = lipgloss.Color("#444444")
	colorSelected = lipgloss.Color("#0085ff")
	colorText     = lipgloss.Color("#ffffff")
	colorSubtext  = lipgloss.Color("#aaaaaa")
	colorError    = lipgloss.Color("#ff4444")
	colorSuccess  = lipgloss.Color("#44ff88")

	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorMuted)

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorPrimary).
			Bold(true).
			Underline(true)

	postStyle = lipgloss.NewStyle().
			Padding(0, 1).
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder)

	selectedPostStyle = lipgloss.NewStyle().
				Padding(0, 1).
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(colorSelected)

	authorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	handleStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	textStyle = lipgloss.NewStyle().
			Foreground(colorText)

	statsStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#222222")).
			Foreground(colorSubtext).
			Padding(0, 1)

	overlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2).
			Background(lipgloss.Color("#111111"))

	composeTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary).
				MarginBottom(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)
)
