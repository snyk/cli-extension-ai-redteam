package clireport

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"

	"github.com/snyk/cli-extension-ai-redteam/internal/models"
)

// RunInteractive launches the interactive TUI report viewer.
// It returns the final static report string for piping when stdout is not a TTY.
func RunInteractive(data *models.ScanReport, meta ScanMeta) error {
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("not a terminal")
	}
	m := newModel(data, meta)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI failed: %w", err)
	}
	return nil
}

// --- bubbletea model ---

type model struct {
	data     *models.ScanReport
	meta     ScanMeta
	cursor   int    // which finding is highlighted
	expanded []bool // which findings are expanded
	scroll   int    // vertical scroll offset
	height   int    // terminal height
	width    int    // terminal width
	showHelp bool   // help overlay visible
}

func newModel(data *models.ScanReport, meta ScanMeta) model {
	expanded := make([]bool, len(data.Results))
	return model{
		data:     data,
		meta:     meta,
		expanded: expanded,
	}
}

//nolint:gocritic // tea.Model interface requires value receiver
func (m model) Init() tea.Cmd {
	return nil
}

//nolint:gocyclo,gocritic,ireturn // inherent complexity; tea.Model interface requires value receiver
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// When help overlay is shown, any key dismisses it.
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

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
		case "?":
			m.showHelp = true
		case "pgup":
			m.scroll -= m.height / 2
			if m.scroll < 0 {
				m.scroll = 0
			}
		case "pgdown":
			m.scroll += m.height / 2
		case "ctrl+up":
			m.scroll -= 3
			if m.scroll < 0 {
				m.scroll = 0
			}
		case "ctrl+down":
			m.scroll += 3
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scroll -= 3
			if m.scroll < 0 {
				m.scroll = 0
			}
		case tea.MouseButtonWheelDown:
			m.scroll += 3
		default:
			// ignore other mouse events
		}
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	}
	return m, nil
}

func (m *model) ensureVisible() {
	// Estimate line position of the current finding.
	// Each collapsed finding ~5 lines, expanded ~25+ lines.
	approxLine := 0
	for i := 0; i < m.cursor; i++ {
		if m.expanded[i] {
			approxLine += 25
		} else {
			approxLine += 5
		}
	}
	// Add header offset (banner + metadata + summary + table).
	approxLine += 20

	viewEnd := m.scroll + m.height
	if approxLine < m.scroll+2 {
		m.scroll = approxLine - 2
	} else if approxLine > viewEnd-8 {
		m.scroll = approxLine - m.height + 12
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

//nolint:gocritic // tea.Model interface requires value receiver
func (m model) View() string {
	// Help overlay takes over the screen.
	if m.showHelp {
		return m.renderHelpOverlay()
	}

	l := newLayout(m.width)
	var sb strings.Builder

	// Banner at the very top.
	sb.WriteString(renderBanner(m.data))

	// Header sections (reuse existing renderers).
	sb.WriteString(renderHeader(m.meta))
	sb.WriteString(renderSummary(m.data))

	if rows := groupByStrategy(m.data.Results); len(rows) > 0 {
		sb.WriteString(renderStrategyTable(rows, l))
	}

	// Interactive findings.
	if len(m.data.Results) > 0 {
		sb.WriteString("  " + headingStyle.Render("Findings"))
		sb.WriteString("  " + dimStyle.Render("(enter: expand/collapse, a: toggle all, ?: help)"))
		sb.WriteString("\n")

		for i, vuln := range m.data.Results {
			sb.WriteString(m.renderFinding(i, vuln, l))
		}
		sb.WriteString("\n")
	}

	if len(m.data.PassedTypes) > 0 {
		sb.WriteString(renderPassedTypes(m.data.PassedTypes))
	}

	// Footer.
	sb.WriteString(renderFooter(m.data))

	// Help bar.
	sb.WriteString(dimStyle.Render("  \u2191\u2193 navigate  enter expand/collapse  a toggle all  ? help  q quit"))
	sb.WriteString("\n")

	content := reportBoxStyle.Width(l.termWidth - 2).Render(sb.String())

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

//nolint:gocritic // tea.Model interface requires value receiver
func (m model) renderHelpOverlay() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	helpContent := strings.Join([]string{
		headingStyle.Render("Keyboard Shortcuts"),
		"",
		valueStyle.Render("  \u2191 / k        ") + dimStyle.Render("Move to previous finding"),
		valueStyle.Render("  \u2193 / j        ") + dimStyle.Render("Move to next finding"),
		valueStyle.Render("  enter / space ") + dimStyle.Render("Expand or collapse selected finding"),
		valueStyle.Render("  a             ") + dimStyle.Render("Toggle all findings open/closed"),
		valueStyle.Render("  e             ") + dimStyle.Render("Expand full evidence (when available)"),
		valueStyle.Render("  ctrl+\u2191/\u2193     ") + dimStyle.Render("Scroll content"),
		valueStyle.Render("  mouse wheel   ") + dimStyle.Render("Scroll content"),
		valueStyle.Render("  pgup / pgdown ") + dimStyle.Render("Scroll half a page"),
		valueStyle.Render("  ?             ") + dimStyle.Render("Toggle this help overlay"),
		valueStyle.Render("  q / esc       ") + dimStyle.Render("Quit"),
		"",
		dimStyle.Render("  Press any key to close this overlay"),
	}, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colPurple).
		Padding(1, 3).
		Width(56).
		Render(helpContent)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

var (
	selectedStyle = lipgloss.NewStyle().
			Foreground(colPurple).
			Bold(true)

	collapsedIcon = dimStyle.Render("\u25b6") // ▶
	expandedIcon  = dimStyle.Render("\u25bc") // ▼
)

//nolint:gocritic // tea.Model interface requires value receiver
func (m model) renderFinding(idx int, vuln models.ReportFinding, l layout) string {
	var sb strings.Builder

	isSelected := idx == m.cursor
	isExpanded := m.expanded[idx]

	icon := collapsedIcon
	if isExpanded {
		icon = expandedIcon
	}

	// Finding header line with severity badge.
	sb.WriteString("\n")
	numStr := fmt.Sprintf("#%d", idx+1)
	name := vuln.Definition.Name
	badge := renderSeverityBadge(vuln.Severity)

	candidateBadge := findingCandidateBadge.Render(" FINDING CANDIDATE ")

	if isSelected {
		fmt.Fprintf(&sb, "  %s %s  %s  %s  %s  %s",
			selectedStyle.Render("\u25b8"),
			icon,
			failText.Render(numStr),
			selectedStyle.Render(name),
			badge,
			candidateBadge,
		)
	} else {
		fmt.Fprintf(&sb, "    %s  %s  %s  %s  %s",
			icon,
			failText.Render(numStr),
			valueStyle.Render(name),
			badge,
			candidateBadge,
		)
	}

	// Contextual keybinding hint for collapsed findings.
	if !isExpanded && isSelected {
		sb.WriteString("  " + dimStyle.Render("[enter] expand"))
	}
	sb.WriteString("\n")

	fmt.Fprintf(&sb, indentFmt, dimStyle.Render(vuln.Definition.Description))

	// OWASP reference inline.
	if len(vuln.Tags) > 0 {
		owasp := owaspLabel(vuln.Tags)
		if owasp != "" {
			fmt.Fprintf(&sb, indentFmt, valueStyle.Render(owasp))
		} else {
			fmt.Fprintf(&sb, "      %s %s\n",
				labelStyle.Render("Tested Vulnerabilities:"),
				dimStyle.Render(strings.Join(vuln.Tags, ", ")),
			)
		}
	}

	if isExpanded {
		sb.WriteString("\n")
		sb.WriteString(renderConversation(vuln.Turns, l))

		if vuln.Evidence.Content.Reason != "" {
			renderEvidenceBlock(&sb, vuln.Evidence.Content.Reason, l)
		}
	}

	return sb.String()
}
