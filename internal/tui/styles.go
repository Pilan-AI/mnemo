package tui

import "github.com/charmbracelet/lipgloss"

// =============================================================================
// UMBRA / MNEMO BRAND COLORS
// =============================================================================
// Deep space theme with memory/neural network accent colors
// "Your AI coding memory, searchable forever"

var (
	// Background gradient simulation (dark to darker)
	Base      = lipgloss.Color("#0d0d14") // Deepest space
	Surface   = lipgloss.Color("#13131f") // Elevated surface
	Overlay   = lipgloss.Color("#1a1a2e") // Cards, panels
	Highlight = lipgloss.Color("#242438") // Hover states

	// UMBRA Brand Colors
	Primary   = lipgloss.Color("#a855f7") // Purple - memory/neural
	Secondary = lipgloss.Color("#6366f1") // Indigo - depth
	Accent    = lipgloss.Color("#22d3ee") // Cyan - active/connected
	Warm      = lipgloss.Color("#f472b6") // Pink - discoveries

	// Text hierarchy
	TextBright = lipgloss.Color("#f8fafc") // Headlines
	Text       = lipgloss.Color("#e2e8f0") // Body
	TextMuted  = lipgloss.Color("#94a3b8") // Secondary
	TextDim    = lipgloss.Color("#475569") // Disabled

	// Semantic
	Success = lipgloss.Color("#4ade80") // Green
	Warning = lipgloss.Color("#fbbf24") // Amber
	Error   = lipgloss.Color("#f87171") // Red

	// Gradient stops for simulated gradients
	GradientStart = lipgloss.Color("#a855f7") // Purple
	GradientMid   = lipgloss.Color("#6366f1") // Indigo
	GradientEnd   = lipgloss.Color("#22d3ee") // Cyan
)

// =============================================================================
// STYLE DEFINITIONS
// =============================================================================

var (
	// Full screen container
	FullScreen = lipgloss.NewStyle().
			Background(Base)

	// Centered content box
	CenteredBox = lipgloss.NewStyle().
			Align(lipgloss.Center).
			AlignVertical(lipgloss.Center)

	// Title styles
	TitleGiant = lipgloss.NewStyle().
			Bold(true).
			Foreground(TextBright).
			MarginBottom(1)

	TitleAccent = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary)

	// Subtitle
	Subtitle = lipgloss.NewStyle().
			Foreground(TextMuted).
			Italic(true)

	// Progress bar styles
	ProgressFilled = lipgloss.NewStyle().
			Foreground(Accent)

	ProgressEmpty = lipgloss.NewStyle().
			Foreground(Highlight)

	// Stat box
	StatBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 3).
		Margin(0, 1)

	StatNumber = lipgloss.NewStyle().
			Bold(true).
			Foreground(Accent)

	StatLabel = lipgloss.NewStyle().
			Foreground(TextMuted)

	// Discovery items
	DiscoveryItem = lipgloss.NewStyle().
			Foreground(Warm).
			PaddingLeft(2)

	// Command hints
	CommandStyle = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	CommandDesc = lipgloss.NewStyle().
			Foreground(TextMuted)

	// Welcome box (glass effect simulation)
	WelcomeBox = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(Primary).
			Padding(2, 4).
			Align(lipgloss.Center)

	// Spinner
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(Accent)

	// Dimmed text for secondary info
	Dimmed = lipgloss.NewStyle().
		Foreground(TextDim)
)

// =============================================================================
// ASCII ART AND DECORATIONS
// =============================================================================

// Large ASCII banner with gradient-like coloring
const BannerWidth = 58

func RenderBanner() string {
	// Each line will be colored differently to simulate gradient
	lines := []struct {
		text  string
		color lipgloss.Color
	}{
		{"", Primary},
		{"  ███╗   ███╗███╗   ██╗███████╗███╗   ███╗ ██████╗ ", Primary},
		{"  ████╗ ████║████╗  ██║██╔════╝████╗ ████║██╔═══██╗", lipgloss.Color("#9333ea")},
		{"  ██╔████╔██║██╔██╗ ██║█████╗  ██╔████╔██║██║   ██║", lipgloss.Color("#7c3aed")},
		{"  ██║╚██╔╝██║██║╚██╗██║██╔══╝  ██║╚██╔╝██║██║   ██║", Secondary},
		{"  ██║ ╚═╝ ██║██║ ╚████║███████╗██║ ╚═╝ ██║╚██████╔╝", lipgloss.Color("#4f46e5")},
		{"  ╚═╝     ╚═╝╚═╝  ╚═══╝╚══════╝╚═╝     ╚═╝ ╚═════╝ ", Accent},
		{"", Accent},
	}

	var result string
	for _, line := range lines {
		style := lipgloss.NewStyle().Foreground(line.color)
		result += style.Render(line.text) + "\n"
	}
	return result
}

// Decorative line with gradient effect
func RenderGradientLine(width int) string {
	chars := []struct {
		char  string
		color lipgloss.Color
	}{
		{"━", Primary},
		{"━", lipgloss.Color("#9333ea")},
		{"━", lipgloss.Color("#7c3aed")},
		{"━", Secondary},
		{"━", lipgloss.Color("#4f46e5")},
		{"━", Accent},
	}

	result := ""
	for i := 0; i < width; i++ {
		idx := (i * len(chars)) / width
		style := lipgloss.NewStyle().Foreground(chars[idx].color)
		result += style.Render(chars[idx].char)
	}
	return result
}

// Spinner frames with neural network theme
var SpinnerFrames = []string{
	"◐", "◓", "◑", "◒",
}

// Alternative braille spinner
var BrailleSpinner = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

// Progress bar characters
const (
	ProgressCharFull  = "█"
	ProgressCharHalf  = "▓"
	ProgressCharLight = "░"
	ProgressCharEmpty = "─"
)

// Box drawing characters for premium feel
const (
	BoxTopLeft     = "╭"
	BoxTopRight    = "╮"
	BoxBottomLeft  = "╰"
	BoxBottomRight = "╯"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
	BoxCross       = "┼"
)

// Decorative characters
const (
	Sparkle   = "✨"
	Brain     = "🧠"
	Rocket    = "🚀"
	Lightning = "⚡"
	Star      = "★"
	Diamond   = "◆"
	Arrow     = "→"
	Check     = "✓"
	Dot       = "•"
)
