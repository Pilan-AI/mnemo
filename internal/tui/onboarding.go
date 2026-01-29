package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// ONBOARDING MODEL
// =============================================================================

type Phase int

const (
	PhaseIntro Phase = iota
	PhaseScanning
	PhaseDiscoveries
	PhaseStats
	PhaseComplete
)

type Discovery struct {
	Project  string
	Messages int
	Icon     string
}

type Stats struct {
	Sessions   int
	Messages   int
	Projects   int
	Days       int
	TopProject string
	TopCount   int
}

type Model struct {
	phase        Phase
	width        int
	height       int
	spinner      spinner.Model
	progress     float64
	discoveries  []Discovery
	stats        Stats
	showIndex    int // For animated reveals
	tickCount    int
	introFade    int // For intro animation
	scanComplete bool

	// Callbacks for actual indexing
	OnIndex func() (Stats, []Discovery)
}

// Messages
type tickMsg time.Time
type scanCompleteMsg struct {
	stats       Stats
	discoveries []Discovery
}

func NewOnboardingModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: BrailleSpinner,
		FPS:    time.Second / 12,
	}
	s.Style = SpinnerStyle

	return Model{
		phase:     PhaseIntro,
		spinner:   s,
		introFade: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.spinner.Tick,
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter", " ":
			return m.advancePhase()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.tickCount++
		return m.handleTick()

	case scanCompleteMsg:
		m.stats = msg.stats
		m.discoveries = msg.discoveries
		m.scanComplete = true
		m.phase = PhaseDiscoveries
		m.showIndex = 0
		return m, tickCmd()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleTick() (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{tickCmd(), m.spinner.Tick}

	switch m.phase {
	case PhaseIntro:
		// Fade in effect
		if m.introFade < 10 {
			m.introFade++
		} else if m.tickCount > 30 { // Auto-advance after ~2.4 seconds
			m.phase = PhaseScanning
			m.tickCount = 0
			// Start actual indexing
			if m.OnIndex != nil {
				go func() {
					stats, discoveries := m.OnIndex()
					// In real implementation, send message back
					_ = stats
					_ = discoveries
				}()
			}
		}

	case PhaseScanning:
		// Simulate progress (in real app, this comes from indexer)
		if m.progress < 1.0 {
			m.progress += 0.015
			if m.progress > 1.0 {
				m.progress = 1.0
			}
		}

	case PhaseDiscoveries:
		// Reveal discoveries one by one
		if m.tickCount%8 == 0 && m.showIndex < len(m.discoveries) {
			m.showIndex++
		}

	case PhaseStats:
		// Animate stat counters (in real implementation)
		if m.showIndex < 4 {
			if m.tickCount%6 == 0 {
				m.showIndex++
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) advancePhase() (tea.Model, tea.Cmd) {
	switch m.phase {
	case PhaseIntro:
		m.phase = PhaseScanning
		m.tickCount = 0
	case PhaseScanning:
		if m.progress >= 1.0 || m.scanComplete {
			m.phase = PhaseDiscoveries
			m.tickCount = 0
			m.showIndex = 0
		}
	case PhaseDiscoveries:
		m.phase = PhaseStats
		m.tickCount = 0
		m.showIndex = 0
	case PhaseStats:
		m.phase = PhaseComplete
		m.tickCount = 0
	case PhaseComplete:
		return m, tea.Quit
	}
	return m, tickCmd()
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var content string

	switch m.phase {
	case PhaseIntro:
		content = m.viewIntro()
	case PhaseScanning:
		content = m.viewScanning()
	case PhaseDiscoveries:
		content = m.viewDiscoveries()
	case PhaseStats:
		content = m.viewStats()
	case PhaseComplete:
		content = m.viewComplete()
	}

	// Center everything
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
		lipgloss.WithWhitespaceBackground(Base),
	)
}

// =============================================================================
// VIEW RENDERERS
// =============================================================================

func (m Model) viewIntro() string {
	var b strings.Builder

	// Animated banner with fade-in effect
	banner := RenderBanner()
	if m.introFade < 10 {
		// Partial fade effect using dimmed colors
		banner = lipgloss.NewStyle().Foreground(TextDim).Render(banner)
	}
	b.WriteString(banner)
	b.WriteString("\n")

	// Tagline with typing effect
	tagline := "Your AI coding memory, searchable forever."
	visible := len(tagline)
	if m.introFade < 10 {
		visible = (len(tagline) * m.introFade) / 10
	}

	taglineStyle := lipgloss.NewStyle().
		Foreground(TextMuted).
		Italic(true).
		Align(lipgloss.Center).
		Width(BannerWidth)

	b.WriteString(taglineStyle.Render(tagline[:visible]))
	b.WriteString("\n\n")

	// Gradient separator
	b.WriteString("    " + RenderGradientLine(50) + "\n\n")

	// Quote
	if m.introFade >= 8 {
		quote := lipgloss.NewStyle().
			Foreground(TextDim).
			Italic(true).
			Align(lipgloss.Center).
			Width(BannerWidth)

		b.WriteString(quote.Render(`"Plan panni pannanum, plan panni pannala na ippudi dhan agum"`))
		b.WriteString("\n")
		b.WriteString(quote.Render("— Pokkiri (2007)"))
		b.WriteString("\n\n")
	}

	// Press to continue
	if m.tickCount%20 < 15 { // Blink effect
		hint := lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true).
			Align(lipgloss.Center).
			Width(BannerWidth)
		b.WriteString(hint.Render("Press ENTER to begin"))
	} else {
		b.WriteString(strings.Repeat(" ", 30))
	}

	return b.String()
}

func (m Model) viewScanning() string {
	var b strings.Builder

	// Mini banner
	title := lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true).
		Align(lipgloss.Center)

	b.WriteString(title.Render("  ◆ MNEMO ◆"))
	b.WriteString("\n\n")

	// Brain animation
	brainFrames := []string{"🧠", "🧠 ", " 🧠", "🧠"}
	brain := brainFrames[m.tickCount%len(brainFrames)]

	statusLine := lipgloss.NewStyle().
		Foreground(Text).
		Align(lipgloss.Center)

	b.WriteString(statusLine.Render(fmt.Sprintf("%s Scanning your AI coding history...", brain)))
	b.WriteString("\n\n")

	// Beautiful progress bar
	b.WriteString(m.renderProgressBar(50, m.progress))
	b.WriteString("\n\n")

	// Spinner with current action
	action := m.spinner.View() + " "
	actions := []string{
		"Discovering Claude Code sessions...",
		"Indexing conversation memories...",
		"Building searchable knowledge base...",
		"Mapping your coding journey...",
		"Finding hidden treasures...",
	}
	actionText := actions[(m.tickCount/15)%len(actions)]

	actionStyle := lipgloss.NewStyle().Foreground(TextMuted)
	b.WriteString(lipgloss.NewStyle().Align(lipgloss.Center).Width(50).Render(
		action + actionStyle.Render(actionText),
	))
	b.WriteString("\n\n")

	// Stats being discovered (animated counters)
	if m.progress > 0.3 {
		statStyle := lipgloss.NewStyle().Foreground(TextDim)
		count := int(float64(m.stats.Sessions) * m.progress)
		if count == 0 && m.progress > 0.1 {
			count = int(m.progress * 100) // Fake progress if no real data yet
		}
		b.WriteString(statStyle.Render(fmt.Sprintf("Sessions found: %d", count)))
	}

	return b.String()
}

func (m Model) renderProgressBar(width int, progress float64) string {
	filled := int(float64(width) * progress)
	empty := width - filled

	// Gradient effect on filled portion
	var bar strings.Builder

	// Left cap
	bar.WriteString(lipgloss.NewStyle().Foreground(Primary).Render("▐"))

	// Filled portion with gradient
	colors := []lipgloss.Color{Primary, lipgloss.Color("#9333ea"), lipgloss.Color("#7c3aed"), Secondary, lipgloss.Color("#4f46e5"), Accent}
	for i := 0; i < filled; i++ {
		colorIdx := (i * len(colors)) / width
		style := lipgloss.NewStyle().Foreground(colors[colorIdx])
		bar.WriteString(style.Render("█"))
	}

	// Empty portion
	emptyStyle := lipgloss.NewStyle().Foreground(Highlight)
	bar.WriteString(emptyStyle.Render(strings.Repeat("░", empty)))

	// Right cap
	bar.WriteString(lipgloss.NewStyle().Foreground(Accent).Render("▌"))

	// Percentage
	pctStyle := lipgloss.NewStyle().Foreground(TextMuted).MarginLeft(2)
	bar.WriteString(pctStyle.Render(fmt.Sprintf("%3.0f%%", progress*100)))

	return lipgloss.NewStyle().Align(lipgloss.Center).Width(width + 10).Render(bar.String())
}

func (m Model) viewDiscoveries() string {
	var b strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Foreground(Warm).
		Bold(true).
		Align(lipgloss.Center).
		MarginBottom(1)

	b.WriteString(title.Render("✨ Discoveries ✨"))
	b.WriteString("\n\n")

	// Discovery list with staggered reveal
	if len(m.discoveries) == 0 {
		// Demo discoveries if none provided
		m.discoveries = []Discovery{
			{Project: "PILAN-INTELLIGENCE-PRISM", Messages: 5420, Icon: "🧠"},
			{Project: "mnemo", Messages: 892, Icon: "💾"},
			{Project: "pilan-tui", Messages: 634, Icon: "🖥️"},
			{Project: "PERSONAL-FORGE", Messages: 2341, Icon: "🔥"},
		}
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Highlight).
		Padding(0, 2).
		Width(50)

	for i, d := range m.discoveries {
		if i >= m.showIndex {
			break
		}

		// Animate entry
		style := boxStyle
		if i == m.showIndex-1 && m.tickCount%4 < 2 {
			style = style.BorderForeground(Warm)
		}

		line := fmt.Sprintf("%s  %s — %s messages",
			d.Icon,
			lipgloss.NewStyle().Foreground(Text).Bold(true).Render(d.Project),
			lipgloss.NewStyle().Foreground(Accent).Render(fmt.Sprintf("%d", d.Messages)),
		)

		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	// Ellipsis for loading more
	if m.showIndex < len(m.discoveries) {
		dots := strings.Repeat(".", (m.tickCount%4)+1)
		b.WriteString(lipgloss.NewStyle().Foreground(TextDim).Render("    " + dots))
	}

	b.WriteString("\n\n")

	// Continue hint
	if m.showIndex >= len(m.discoveries) {
		hint := lipgloss.NewStyle().Foreground(TextMuted)
		b.WriteString(hint.Render("Press ENTER to see your stats"))
	}

	return b.String()
}

func (m Model) viewStats() string {
	var b strings.Builder

	// Title with gradient
	b.WriteString(RenderGradientLine(50))
	b.WriteString("\n\n")

	title := lipgloss.NewStyle().
		Foreground(TextBright).
		Bold(true).
		Align(lipgloss.Center)

	b.WriteString(title.Render("Your Memory at a Glance"))
	b.WriteString("\n\n")

	// Demo stats if none provided
	if m.stats.Sessions == 0 {
		m.stats = Stats{
			Sessions:   1999,
			Messages:   20896,
			Projects:   47,
			Days:       180,
			TopProject: "PILAN-INTELLIGENCE-PRISM",
			TopCount:   5420,
		}
	}

	// Stat boxes in a row
	statBoxes := []struct {
		number string
		label  string
		icon   string
		color  lipgloss.Color
	}{
		{fmt.Sprintf("%d", m.stats.Sessions), "sessions", "📚", Primary},
		{fmt.Sprintf("%d", m.stats.Messages), "messages", "💬", Accent},
		{fmt.Sprintf("%d", m.stats.Projects), "projects", "📁", Secondary},
		{fmt.Sprintf("%d", m.stats.Days), "days", "📅", Warm},
	}

	var statRow strings.Builder
	for i, s := range statBoxes {
		if i >= m.showIndex {
			// Not yet revealed - show placeholder
			box := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Highlight).
				Padding(1, 2).
				Width(14).
				Align(lipgloss.Center)
			statRow.WriteString(box.Render("..."))
			continue
		}

		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(s.color).
			Padding(1, 2).
			Width(14).
			Align(lipgloss.Center)

		numberStyle := lipgloss.NewStyle().Foreground(s.color).Bold(true)
		labelStyle := lipgloss.NewStyle().Foreground(TextMuted)

		content := s.icon + "\n" +
			numberStyle.Render(s.number) + "\n" +
			labelStyle.Render(s.label)

		statRow.WriteString(box.Render(content))
	}

	b.WriteString(lipgloss.NewStyle().Align(lipgloss.Center).Render(statRow.String()))
	b.WriteString("\n\n")

	// Top project callout
	if m.showIndex >= 4 {
		highlight := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(Primary).
			Padding(1, 3).
			Align(lipgloss.Center)

		content := fmt.Sprintf("🏆 Your most active project:\n%s\n%s messages",
			lipgloss.NewStyle().Foreground(TextBright).Bold(true).Render(m.stats.TopProject),
			lipgloss.NewStyle().Foreground(Accent).Render(fmt.Sprintf("%d", m.stats.TopCount)),
		)

		b.WriteString(highlight.Render(content))
		b.WriteString("\n\n")

		hint := lipgloss.NewStyle().Foreground(TextMuted)
		b.WriteString(hint.Render("Press ENTER to continue"))
	}

	return b.String()
}

func (m Model) viewComplete() string {
	var b strings.Builder

	// Celebration banner
	banner := lipgloss.NewStyle().
		Foreground(Success).
		Bold(true).
		Align(lipgloss.Center)

	b.WriteString(banner.Render("✓ Your memory is ready."))
	b.WriteString("\n\n")

	b.WriteString(RenderGradientLine(50))
	b.WriteString("\n\n")

	// Commands section
	title := lipgloss.NewStyle().
		Foreground(TextBright).
		Bold(true)

	b.WriteString(title.Render("  Try these commands:"))
	b.WriteString("\n\n")

	commands := []struct {
		cmd  string
		desc string
	}{
		{"mnemo search \"<anything>\"", "Search your entire history"},
		{"mnemo recent", "See recent sessions"},
		{"mnemo context <project>", "Get project context"},
		{"mnemo add ~/path", "Add more knowledge"},
	}

	for _, c := range commands {
		cmdStyle := lipgloss.NewStyle().Foreground(Accent).Bold(true).Width(30)
		descStyle := lipgloss.NewStyle().Foreground(TextMuted)

		b.WriteString("    ")
		b.WriteString(cmdStyle.Render(c.cmd))
		b.WriteString(descStyle.Render(c.desc))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(RenderGradientLine(50))
	b.WriteString("\n\n")

	// Final quote
	quote := lipgloss.NewStyle().
		Foreground(TextDim).
		Italic(true).
		Align(lipgloss.Center).
		Width(50)

	b.WriteString(quote.Render("Don't be the Lochak-Mochak engineer."))
	b.WriteString("\n")
	b.WriteString(quote.Render("Be the one who ships."))
	b.WriteString("\n\n")

	// Exit hint
	hint := lipgloss.NewStyle().Foreground(TextMuted)
	b.WriteString(hint.Render("      Press ENTER or Q to exit"))

	return b.String()
}

// =============================================================================
// PUBLIC API
// =============================================================================

// RunOnboarding starts the interactive onboarding experience
func RunOnboarding(onIndex func() (Stats, []Discovery)) error {
	m := NewOnboardingModel()
	m.OnIndex = onIndex

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// SetStats allows external code to set stats for display
func (m *Model) SetStats(stats Stats) {
	m.stats = stats
}

// SetDiscoveries allows external code to set discoveries for display
func (m *Model) SetDiscoveries(discoveries []Discovery) {
	m.discoveries = discoveries
}

// MarkScanComplete signals that scanning is done
func (m *Model) MarkScanComplete() {
	m.scanComplete = true
	m.progress = 1.0
}
