package ui

import "github.com/charmbracelet/lipgloss"

type themeColors struct {
	Primary    lipgloss.Color
	Muted      lipgloss.Color
	Border     lipgloss.Color
	Selected   lipgloss.Color
	SelectedBG lipgloss.Color
	Text       lipgloss.Color
	Subtext    lipgloss.Color
	Error      lipgloss.Color
	Success    lipgloss.Color
	StatusBG   lipgloss.Color
	OverlayBG  lipgloss.Color
}

var themes = map[string]themeColors{
	"tokyonight": {
		Primary:    "#7aa2f7",
		Muted:      "#565f89",
		Border:     "#414868",
		Selected:   "#7aa2f7",
		SelectedBG: "#1e2030",
		Text:       "#c0caf5",
		Subtext:    "#9aa5ce",
		Error:      "#f7768e",
		Success:    "#9ece6a",
		StatusBG:   "#1a1b26",
		OverlayBG:  "#16161e",
	},
	"kanagawa": {
		Primary:    "#7e9cd8",
		Muted:      "#727169",
		Border:     "#54546d",
		Selected:   "#7e9cd8",
		SelectedBG: "#1e2030",
		Text:       "#dcd7ba",
		Subtext:    "#938aa9",
		Error:      "#e82424",
		Success:    "#76946a",
		StatusBG:   "#1f1f28",
		OverlayBG:  "#16161d",
	},
	"solarized": {
		Primary:    "#268bd2",
		Muted:      "#586e75",
		Border:     "#073642",
		Selected:   "#268bd2",
		SelectedBG: "#0a3040",
		Text:       "#839496",
		Subtext:    "#657b83",
		Error:      "#dc322f",
		Success:    "#859900",
		StatusBG:   "#073642",
		OverlayBG:  "#002b36",
	},
	"catppuccin": {
		Primary:    "#89b4fa",
		Muted:      "#6c7086",
		Border:     "#45475a",
		Selected:   "#89b4fa",
		SelectedBG: "#2a2b3c",
		Text:       "#cdd6f4",
		Subtext:    "#a6adc8",
		Error:      "#f38ba8",
		Success:    "#a6e3a1",
		StatusBG:   "#313244",
		OverlayBG:  "#1e1e2e",
	},
}

var (
	colorPrimary  lipgloss.Color
	colorMuted    lipgloss.Color
	colorBorder   lipgloss.Color
	colorSelected lipgloss.Color
	colorText     lipgloss.Color
	colorSubtext  lipgloss.Color
	colorError    lipgloss.Color
	colorSuccess  lipgloss.Color

	tabStyle          lipgloss.Style
	activeTabStyle    lipgloss.Style
	postStyle         lipgloss.Style
	selectedPostStyle lipgloss.Style
	authorStyle       lipgloss.Style
	handleStyle       lipgloss.Style
	textStyle         lipgloss.Style
	statsStyle        lipgloss.Style
	statusBarStyle    lipgloss.Style
	overlayStyle      lipgloss.Style
	composeTitleStyle lipgloss.Style
	errorStyle        lipgloss.Style
	successStyle      lipgloss.Style
)

func applyTheme(name string) {
	t, ok := themes[name]
	if !ok {
		t = themes["tokyonight"]
	}

	colorPrimary = t.Primary
	colorMuted = t.Muted
	colorBorder = t.Border
	colorSelected = t.Selected
	colorText = t.Text
	colorSubtext = t.Subtext
	colorError = t.Error
	colorSuccess = t.Success

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
		BorderForeground(colorSelected).
		Background(t.SelectedBG)

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
		Background(t.StatusBG).
		Foreground(colorSubtext).
		Padding(0, 1)

	overlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Background(t.OverlayBG)

	composeTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		MarginBottom(1)

	errorStyle = lipgloss.NewStyle().
		Foreground(colorError)

	successStyle = lipgloss.NewStyle().
		Foreground(colorSuccess)
}
