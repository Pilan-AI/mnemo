// Package tui provides the terminal UI components for mnemo using Bubble Tea
// and Lip Gloss. It includes the first-run onboarding experience and the
// interactive project selector.
package tui

import "github.com/charmbracelet/lipgloss"

// =============================================================================
// MNEMO BRAND COLORS
// =============================================================================
// Deep space theme with cyan accent spectrum.
// Designed for dark terminal backgrounds.

var (
	// Background gradient simulation (dark to darker)
	Base      = lipgloss.Color("#0d0d14") // Deepest space
	Surface   = lipgloss.Color("#13131f") // Elevated surface
	Overlay   = lipgloss.Color("#1a1a2e") // Cards, panels
	Highlight = lipgloss.Color("#242438") // Hover states

	// MNEMO Brand Colors (Cyan Spectrum)
	Primary   = lipgloss.Color("#00D9FF") // Cyan Bright - brand
	Secondary = lipgloss.Color("#0A99B5") // Cyan Mid - depth
	Accent    = lipgloss.Color("#2DD4BF") // Aqua - active/connected
	Warm      = lipgloss.Color("#2DD4BF") // Aqua - discoveries

	// Text hierarchy
	TextBright = lipgloss.Color("#f8fafc") // Headlines
	Text       = lipgloss.Color("#e2e8f0") // Body
	TextMuted  = lipgloss.Color("#94a3b8") // Secondary
	TextDim    = lipgloss.Color("#475569") // Disabled

	// Semantic
	Success = lipgloss.Color("#4ade80") // Green
	Warning = lipgloss.Color("#fbbf24") // Amber
	Error   = lipgloss.Color("#f87171") // Red

	// Gradient stops for simulated gradients (Cyan spectrum)
	GradientStart = lipgloss.Color("#00D9FF") // Cyan Bright
	GradientMid   = lipgloss.Color("#0A99B5") // Cyan Mid
	GradientEnd   = lipgloss.Color("#0D3D50") // Cyan Deep
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
const BannerWidth = 21

func RenderBanner() string {
	lines := []struct {
		text  string
		color lipgloss.Color
	}{
		{"", Primary},
		{"  ┏┳┓ ┏┓╻ ┏━╸ ┏┳┓ ┏━┓", Primary},
		{"  ┃┃┃ ┃┗┫ ┣╸  ┃┃┃ ┃ ┃", Secondary},
		{"  ╹ ╹ ╹ ╹ ┗━╸ ╹ ╹ ┗━┛", Accent},
		{"", Accent},
	}

	var result string
	for _, line := range lines {
		style := lipgloss.NewStyle().Foreground(line.color)
		result += style.Render(line.text) + "\n"
	}
	return result
}

// Decorative line with cyan gradient effect
func RenderGradientLine(width int) string {
	chars := []struct {
		char  string
		color lipgloss.Color
	}{
		{"━", Primary},
		{"━", lipgloss.Color("#00C4E6")},
		{"━", lipgloss.Color("#0A99B5")},
		{"━", Secondary},
		{"━", lipgloss.Color("#087A8F")},
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
	Sparkle   = "✦"
	Brain     = "◈"
	Rocket    = "▸"
	Lightning = "↯"
	Star      = "★"
	Diamond   = "◆"
	Arrow     = "→"
	Check     = "✓"
	Dot       = "•"
)
