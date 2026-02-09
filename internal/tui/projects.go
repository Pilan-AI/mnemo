// projects.go implements the interactive project selector TUI using Bubble Tea.
// Users can toggle projects on/off for selective indexing during onboarding.
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProjectItem represents a single project in the selector list.
type ProjectItem struct {
	Path         string
	Name         string
	LastActivity time.Time
	Status       string
	Selected     bool
}

// ProjectSelectorModel is the Bubble Tea model for the interactive project selector.
type ProjectSelectorModel struct {
	active      []ProjectItem
	inactive    []ProjectItem
	cursor      int
	width       int
	height      int
	done        bool
	focusActive bool

	OnComplete func(enabled []string, disabled []string)
}

// NewProjectSelectorModel creates a new project selector with active projects pre-selected.
func NewProjectSelectorModel(active, inactive []ProjectItem) ProjectSelectorModel {
	for i := range active {
		active[i].Selected = true
	}
	for i := range inactive {
		inactive[i].Selected = false
	}

	return ProjectSelectorModel{
		active:      active,
		inactive:    inactive,
		cursor:      0,
		focusActive: true,
	}
}

func (m ProjectSelectorModel) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (m ProjectSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else if !m.focusActive && len(m.active) > 0 {
				m.focusActive = true
				m.cursor = len(m.active) - 1
			}

		case "down", "j":
			currentList := m.active
			if !m.focusActive {
				currentList = m.inactive
			}
			if m.cursor < len(currentList)-1 {
				m.cursor++
			} else if m.focusActive && len(m.inactive) > 0 {
				m.focusActive = false
				m.cursor = 0
			}

		case "tab":
			if m.focusActive && len(m.inactive) > 0 {
				m.focusActive = false
				m.cursor = 0
			} else if !m.focusActive && len(m.active) > 0 {
				m.focusActive = true
				m.cursor = 0
			}

		case " ", "x":
			if m.focusActive && m.cursor < len(m.active) {
				m.active[m.cursor].Selected = !m.active[m.cursor].Selected
			} else if !m.focusActive && m.cursor < len(m.inactive) {
				m.inactive[m.cursor].Selected = !m.inactive[m.cursor].Selected
			}

		case "a":
			for i := range m.active {
				m.active[i].Selected = true
			}

		case "n":
			for i := range m.active {
				m.active[i].Selected = false
			}
			for i := range m.inactive {
				m.inactive[i].Selected = false
			}

		case "enter":
			m.done = true
			if m.OnComplete != nil {
				var enabled, disabled []string
				for _, p := range m.active {
					if p.Selected {
						enabled = append(enabled, p.Path)
					} else {
						disabled = append(disabled, p.Path)
					}
				}
				for _, p := range m.inactive {
					if p.Selected {
						enabled = append(enabled, p.Path)
					} else {
						disabled = append(disabled, p.Path)
					}
				}
				m.OnComplete(enabled, disabled)
			}
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m ProjectSelectorModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("  ◆ MNEMO — Project Selection ◆"))
	b.WriteString("\n\n")

	subtitleStyle := lipgloss.NewStyle().Foreground(TextMuted)
	b.WriteString(subtitleStyle.Render("  Select which projects to track for AI session indexing:"))
	b.WriteString("\n\n")

	if len(m.active) > 0 {
		sectionStyle := lipgloss.NewStyle().Foreground(Success).Bold(true)
		b.WriteString(sectionStyle.Render("  ACTIVE (last 60 days)"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(Highlight).Render("  " + strings.Repeat("─", 50)))
		b.WriteString("\n")

		for i, p := range m.active {
			b.WriteString(m.renderProjectItem(p, m.focusActive && i == m.cursor))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(m.inactive) > 0 {
		sectionStyle := lipgloss.NewStyle().Foreground(Warm).Bold(true)
		b.WriteString(sectionStyle.Render("  INACTIVE (60-90 days)"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(Highlight).Render("  " + strings.Repeat("─", 50)))
		b.WriteString("\n")

		for i, p := range m.inactive {
			b.WriteString(m.renderProjectItem(p, !m.focusActive && i == m.cursor))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(m.active) == 0 && len(m.inactive) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(TextDim).Italic(true)
		b.WriteString(emptyStyle.Render("  No projects discovered yet. Run 'mnemo index' first."))
		b.WriteString("\n\n")
	}

	b.WriteString("\n")
	b.WriteString(RenderGradientLine(50))
	b.WriteString("\n\n")

	helpStyle := lipgloss.NewStyle().Foreground(TextDim)
	b.WriteString(helpStyle.Render("  [↑/↓] Navigate  [Space] Toggle  [a] Select All Active  [n] Deselect All"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [Tab] Switch Section  [Enter] Confirm  [q] Quit"))

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		b.String(),
		lipgloss.WithWhitespaceBackground(Base),
	)
}

func (m ProjectSelectorModel) renderProjectItem(p ProjectItem, isCursor bool) string {
	checkbox := "[ ]"
	checkStyle := lipgloss.NewStyle().Foreground(TextDim)
	if p.Selected {
		checkbox = "[✓]"
		checkStyle = lipgloss.NewStyle().Foreground(Success)
	}

	cursor := "   "
	if isCursor {
		cursor = " ► "
	}

	cursorStyle := lipgloss.NewStyle().Foreground(Accent)
	nameStyle := lipgloss.NewStyle().Foreground(Text)
	if isCursor {
		nameStyle = nameStyle.Bold(true).Foreground(TextBright)
	}

	timeAgo := formatTimeAgo(p.LastActivity)
	timeStyle := lipgloss.NewStyle().Foreground(TextDim)

	name := p.Name
	if len(name) > 35 {
		name = name[:32] + "..."
	}

	return fmt.Sprintf("%s%s %-38s %s",
		cursorStyle.Render(cursor),
		checkStyle.Render(checkbox),
		nameStyle.Render(name),
		timeStyle.Render("("+timeAgo+")"),
	)
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	days := int(time.Since(t).Hours() / 24)
	if days == 0 {
		return "today"
	} else if days == 1 {
		return "yesterday"
	} else if days < 7 {
		return fmt.Sprintf("%d days ago", days)
	} else if days < 30 {
		weeks := days / 7
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if days < 365 {
		months := days / 30
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
	return fmt.Sprintf("%d days ago", days)
}

func RunProjectSelector(active, inactive []ProjectItem, onComplete func(enabled []string, disabled []string)) error {
	m := NewProjectSelectorModel(active, inactive)
	m.OnComplete = onComplete

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
