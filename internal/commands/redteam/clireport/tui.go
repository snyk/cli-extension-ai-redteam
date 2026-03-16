package clireport

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/snyk/cli-extension-ai-redteam/internal/models"
)

// RunInteractive launches the interactive TUI report viewer.
// It returns the final static report string for piping when stdout is not a TTY.
func RunInteractive(data *models.GetAIVulnerabilitiesResponseData, meta ScanMeta) error {
	m := newModel(data, meta)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// --- bubbletea model ---

type model struct {
	data     *models.GetAIVulnerabilitiesResponseData
	meta     ScanMeta
	cursor   int    // which finding is highlighted
	expanded []bool // which findings are expanded
	scroll   int    // vertical scroll offset
	height   int    // terminal height
	width    int    // terminal width
}

func newModel(data *models.GetAIVulnerabilitiesResponseData, meta ScanMeta) model {
	expanded := make([]bool, len(data.Results))
	return model{
		data:     data,
		meta:     meta,
		expanded: expanded,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		case "down", "j":
			if m.cursor < len(m.data.Results)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "enter", " ":
			if m.cursor < len(m.expanded) {
				m.expanded[m.cursor] = !m.expanded[m.cursor]
			}
		case "a":
			// Toggle all
			allOpen := true
			for _, e := range m.expanded {
				if !e {
					allOpen = false
					break
				}
			}
			for i := range m.expanded {
				m.expanded[i] = !allOpen
			}
		case "pgup":
			m.scroll -= m.height / 2
			if m.scroll < 0 {
				m.scroll = 0
			}
		case "pgdown":
			m.scroll += m.height / 2
		}
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	}
	return m, nil
}

func (m *model) ensureVisible() {
	// Simple heuristic: scroll so cursor finding is roughly visible.
	// Each collapsed finding ~4 lines, expanded ~15+ lines.
	approxLine := 0
	for i := 0; i < m.cursor; i++ {
		if m.expanded[i] {
			approxLine += 15
		} else {
			approxLine += 4
		}
	}
	// Add header offset (~15 lines for metadata + summary + table header).
	approxLine += 15

	viewEnd := m.scroll + m.height
	if approxLine < m.scroll {
		m.scroll = approxLine - 2
	} else if approxLine > viewEnd-5 {
		m.scroll = approxLine - m.height + 10
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m model) View() string {
	var sb strings.Builder

	// Header sections (reuse existing renderers).
	sb.WriteString(renderHeader(m.meta))
	sb.WriteString(renderSummary(m.data))

	if m.data.Summary != nil && len(m.data.Summary.Vulnerabilities) > 0 {
		sb.WriteString(renderStrategyTable(m.data.Summary))
	}

	// Interactive findings.
	if len(m.data.Results) > 0 {
		sb.WriteString("  " + headingStyle.Render("Findings"))
		sb.WriteString("  " + dimStyle.Render("(enter: expand/collapse, a: toggle all)"))
		sb.WriteString("\n")

		for i, vuln := range m.data.Results {
			sb.WriteString(m.renderFinding(i, vuln))
		}
		sb.WriteString("\n")
	}

	// Footer.
	sb.WriteString(renderFooter(m.data))

	// Help bar.
	sb.WriteString(dimStyle.Render("  \u2191\u2193 navigate  enter expand/collapse  a toggle all  q quit"))
	sb.WriteString("\n")

	content := reportBoxStyle.Render(sb.String())

	// Apply scrolling.
	lines := strings.Split(content, "\n")
	if m.scroll > len(lines) {
		m.scroll = len(lines) - 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}

	visibleHeight := m.height
	if visibleHeight <= 0 {
		visibleHeight = 40
	}

	end := m.scroll + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}
	start := m.scroll
	if start > len(lines) {
		start = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

var (
	selectedStyle = lipgloss.NewStyle().
			Foreground(colPurple).
			Bold(true)

	collapsedIcon = dimStyle.Render("\u25b6") // ▶
	expandedIcon  = dimStyle.Render("\u25bc") // ▼
)

func (m model) renderFinding(idx int, vuln models.AIVulnerability) string {
	var sb strings.Builder

	isSelected := idx == m.cursor
	isExpanded := m.expanded[idx]

	icon := collapsedIcon
	if isExpanded {
		icon = expandedIcon
	}

	// Finding header line.
	sb.WriteString("\n")
	numStr := fmt.Sprintf("#%d", idx+1)
	name := vuln.Definition.Name

	if isSelected {
		sb.WriteString(fmt.Sprintf("  %s %s  %s  %s\n",
			selectedStyle.Render("\u25b8"),
			icon,
			failText.Render(numStr),
			selectedStyle.Render(name),
		))
	} else {
		sb.WriteString(fmt.Sprintf("    %s  %s  %s\n",
			icon,
			failText.Render(numStr),
			valueStyle.Render(name),
		))
	}

	sb.WriteString(fmt.Sprintf("      %s\n", dimStyle.Render(vuln.Definition.Description)))

	pass, total := findingPassRate(vuln.Definition.ID, m.data.Summary)
	sb.WriteString(fmt.Sprintf("      %s  %s\n",
		labelStyle.Render("Pass Rate"),
		dimStyle.Render(fmt.Sprintf("%d / %d tests passed", pass, total)),
	))

	if len(vuln.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("      %s %s\n",
			labelStyle.Render("Tested Vulnerabilities:"),
			dimStyle.Render(strings.Join(vuln.Tags, ", ")),
		))
	}

	if isExpanded {
		sb.WriteString("\n")
		sb.WriteString(renderConversation(vuln.Turns, true))

		if vuln.Evidence.Content.Reason != "" {
			evidence := vuln.Evidence.Content.Reason
			if len(evidence) > maxEvidenceLen {
				evidence = evidence[:maxEvidenceLen] + "..."
			}
			content := fmt.Sprintf("%s  %s", labelStyle.Render("Evidence"), dimStyle.Render(evidence))
			sb.WriteString(evidenceBoxStyle.Render(content))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
