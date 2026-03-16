package clireport

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/snyk/cli-extension-ai-redteam/internal/models"
)

const (
	maxEvidenceLen  = 300
	maxConvoLen     = 300
	defaultTermWidth = 100

	// Table column widths — used by both header and data rows.
	colStrategy = 45
	colSeverity = 13
	colPassW    = 6
	colFailW    = 6
	colRate     = 11
	colOwasp    = 26
)

// Total table width: all columns + 2-space gaps between each.
const tableWidth = colStrategy + colSeverity + colPassW + colFailW + colRate + colOwasp + 10 // 5 gaps * 2

// ScanMeta holds metadata about the scan for display in the report header.
type ScanMeta struct {
	TargetURL        string
	Goal             string
	Strategies       []string
	FullConversation bool
}

// Render produces a styled CLI report from normalized scan results.
func Render(data *models.GetAIVulnerabilitiesResponseData, meta ScanMeta) string {
	var sb strings.Builder

	sb.WriteString(renderHeader(meta))
	sb.WriteString(renderSummary(data))

	if data.Summary != nil && len(data.Summary.Vulnerabilities) > 0 {
		sb.WriteString(renderStrategyTable(data.Summary))
	}

	if len(data.Results) > 0 {
		sb.WriteString(renderFindings(data.Results, data.Summary, meta.FullConversation))
	}

	sb.WriteString(renderFooter(data))

	return reportBoxStyle.Render(sb.String()) + "\n"
}

// --- palette ---

var (
	colPurple = lipgloss.Color("#CBABEE") // headlines + agent text
	colRed    = lipgloss.Color("#E44A50") // all red: fail, high sev, etc.
	colMedSev = lipgloss.Color("#b7950b")
	colLowSev = lipgloss.Color("#1a5276")
	colPass   = lipgloss.Color("#7BF1A8")
	colSlate  = lipgloss.Color("#5d6d7e")
	colDim    = lipgloss.Color("#7f8c8d")
	colWhite  = lipgloss.Color("#FFFFFF")

	headingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colPurple)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colWhite)

	labelStyle = lipgloss.NewStyle().
			Foreground(colSlate)

	valueStyle = lipgloss.NewStyle().
			Foreground(colWhite)

	dimStyle = lipgloss.NewStyle().
			Foreground(colDim)

	passText = lipgloss.NewStyle().
			Foreground(colPass)

	failText = lipgloss.NewStyle().
			Foreground(colRed)

	purpleText = lipgloss.NewStyle().
			Foreground(colPurple)

	reportBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colSlate).
			Padding(1, 2)

	userBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colSlate).
			Padding(0, 1).
			MarginLeft(6).
			Width(70)

	agentBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colPurple).
			Padding(0, 1).
			MarginLeft(6).
			Width(70)

	evidenceBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colSlate).
				Padding(0, 1).
				MarginLeft(6).
				Width(70)
)

// --- header ---

func renderHeader(meta ScanMeta) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("  " + headingStyle.Render("Scan Metadata"))
	sb.WriteString("\n\n")
	sb.WriteString("    " + labelStyle.Render("Target:     ") + valueStyle.Render(meta.TargetURL) + "\n")
	sb.WriteString("    " + labelStyle.Render("Goal:       ") + valueStyle.Render(meta.Goal) + "\n")
	sb.WriteString("    " + labelStyle.Render("Strategies: ") + valueStyle.Render(strings.Join(meta.Strategies, ", ")) + "\n")
	sb.WriteString("\n")

	return sb.String()
}

// --- summary ---

func renderSummary(data *models.GetAIVulnerabilitiesResponseData) string {
	var passed, failed, skipped int

	if data.Summary != nil {
		for _, v := range data.Summary.Vulnerabilities {
			if v.Vulnerable {
				failed++
			} else if v.Status == "completed" {
				passed++
			} else {
				skipped++
			}
		}
	}

	var sb strings.Builder

	sb.WriteString("  " + headingStyle.Render("Summary"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("    %s %d passed    %s %d failed    %s %d skipped\n\n",
		passText.Render("\u2713"), passed,
		failText.Render("\u2715"), failed,
		dimStyle.Render("\u25cc"), skipped,
	))

	return sb.String()
}

// --- strategy table ---

func renderStrategyTable(summary *models.AIScanSummary) string {
	var sb strings.Builder

	sb.WriteString("  " + headingStyle.Render("Strategy Breakdown"))
	sb.WriteString("\n\n")

	dash := dimStyle.Render(strings.Repeat("- ", tableWidth/2))

	// Top border
	sb.WriteString("    " + dash + "\n")

	// Column header — uses the exact same widths as data rows.
	sb.WriteString(fmt.Sprintf("    %-*s  %-*s  %*s  %*s  %*s  %-*s\n",
		colStrategy, dimStyle.Render("STRATEGY"),
		colSeverity, dimStyle.Render("SEVERITY"),
		colPassW, dimStyle.Render("PASS"),
		colFailW, dimStyle.Render("FAIL"),
		colRate, dimStyle.Render("PASS RATE"),
		colOwasp, dimStyle.Render("TESTED VULNERABILITIES"),
	))
	sb.WriteString("    " + dash + "\n")

	for _, v := range summary.Vulnerabilities {
		leaf := truncateCol(leafStrategy(v.EngineTag), colStrategy)
		sev := renderSeverityText(v.Severity)
		tags := truncateCol(formatTags(v.EngineTag), colOwasp)

		pass := v.TotalChats - v.Successful
		fail := v.Successful
		rate := passRate(pass, fail)

		sb.WriteString(fmt.Sprintf("    %-*s  %-*s  %*d  %*d  %*s  %-*s\n",
			colStrategy, valueStyle.Render(leaf),
			colSeverity, sev,
			colPassW, pass,
			colFailW, fail,
			colRate, rate,
			colOwasp, dimStyle.Render(tags),
		))
		// Full ID on next line, indented and dim.
		sb.WriteString(fmt.Sprintf("      %s\n", dimStyle.Render(v.EngineTag)))
	}

	// Bottom border
	sb.WriteString("    " + dash + "\n")
	return sb.String()
}

func leafStrategy(engineTag string) string {
	parts := strings.Split(engineTag, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return engineTag
}

func truncateCol(s string, max int) string {
	if len(s) > max {
		return s[:max-1] + "\u2026"
	}
	return s
}

func renderSeverityText(sev string) string {
	upper := strings.ToUpper(sev)
	switch strings.ToLower(sev) {
	case "critical", "high":
		return lipgloss.NewStyle().Foreground(colRed).Render(upper)
	case "medium":
		return lipgloss.NewStyle().Foreground(colMedSev).Render(upper)
	case "low":
		return lipgloss.NewStyle().Foreground(colLowSev).Render(upper)
	default:
		return lipgloss.NewStyle().Foreground(colMedSev).Render(upper)
	}
}

func passRate(pass, fail int) string {
	total := pass + fail
	if total == 0 {
		return dimStyle.Render("--")
	}
	pct := pass * 100 / total
	s := fmt.Sprintf("%d%%", pct)
	if pct >= 50 {
		return passText.Render(s)
	}
	return failText.Render(s)
}

func formatTags(engineTag string) string {
	parts := strings.Split(engineTag, "/")
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

// --- findings ---

func renderFindings(results []models.AIVulnerability, summary *models.AIScanSummary, fullConversation bool) string {
	var sb strings.Builder

	sb.WriteString("  " + headingStyle.Render("Findings"))
	sb.WriteString("\n")

	for i, vuln := range results {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("    %s  %s\n",
			failText.Render(fmt.Sprintf("! #%d", i+1)),
			valueStyle.Render(vuln.Definition.Name),
		))
		sb.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render(vuln.Definition.Description)))

		pass, total := findingPassRate(vuln.Definition.ID, summary)
		sb.WriteString(fmt.Sprintf("    %s  %s\n",
			labelStyle.Render("Pass Rate"),
			dimStyle.Render(fmt.Sprintf("%d / %d tests passed", pass, total)),
		))

		if len(vuln.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("    %s %s\n",
				labelStyle.Render("Tested Vulnerabilities:"),
				dimStyle.Render(strings.Join(vuln.Tags, ", ")),
			))
		}

		if fullConversation {
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
		} else {
			sb.WriteString(fmt.Sprintf("    %s\n",
				dimStyle.Render("[use --full-conversation to expand chat details]")))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

func findingPassRate(defID string, summary *models.AIScanSummary) (pass, total int) {
	if summary == nil {
		return 0, 1
	}
	for _, v := range summary.Vulnerabilities {
		if v.Slug == defID {
			total = v.TotalChats
			pass = total - v.Successful
			return pass, total
		}
	}
	return 0, 1
}

// renderConversation renders turns as a chat-style layout.
// By default only the first and last turns are shown. Pass fullConversation=true
// to display all turns (--full-conversation flag).
func renderConversation(turns []models.Turn, fullConversation bool) string {
	visible := turns
	omitted := 0

	if !fullConversation && len(turns) > 2 {
		omitted = len(turns) - 2
		visible = []models.Turn{turns[0], turns[len(turns)-1]}
	}

	var sb strings.Builder

	for i, turn := range visible {
		if turn.Request != nil {
			msg := truncate(*turn.Request, maxConvoLen)
			content := fmt.Sprintf("%s  %s", dimStyle.Render("You"), valueStyle.Render(msg))
			sb.WriteString(userBoxStyle.Render(content))
			sb.WriteString("\n")
		}
		if turn.Response != nil {
			msg := truncate(*turn.Response, maxConvoLen)
			content := fmt.Sprintf("%s  %s", purpleText.Render("Agent"), purpleText.Render(msg))
			sb.WriteString(agentBoxStyle.Render(content))
			sb.WriteString("\n")
		}

		if i == 0 && omitted > 0 {
			noun := "turn"
			if omitted > 1 {
				noun = "turns"
			}
			sb.WriteString(fmt.Sprintf("      %s\n",
				dimStyle.Render(fmt.Sprintf("... %d %s hidden (use --full-conversation to expand)", omitted, noun))))
		}
	}

	return sb.String()
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "\u2026"
	}
	return s
}

// --- footer ---

func renderFooter(data *models.GetAIVulnerabilitiesResponseData) string {
	counts := countBySeverity(data.Results)

	var sb strings.Builder

	if len(data.Results) == 0 {
		sb.WriteString("\n    " + passText.Render("\u2713 No vulnerabilities found"))
		sb.WriteString("\n\n")
		return sb.String()
	}

	var parts []string
	if n, ok := counts["critical"]; ok && n > 0 {
		parts = append(parts, failText.Render(fmt.Sprintf("%d critical", n)))
	}
	if n, ok := counts["high"]; ok && n > 0 {
		parts = append(parts, failText.Render(fmt.Sprintf("%d high", n)))
	}
	if n, ok := counts["medium"]; ok && n > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(colMedSev).Render(fmt.Sprintf("%d medium", n)))
	}
	if n, ok := counts["low"]; ok && n > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(colLowSev).Render(fmt.Sprintf("%d low", n)))
	}

	total := len(data.Results)
	noun := "vulnerability"
	if total > 1 {
		noun = "vulnerabilities"
	}

	sb.WriteString(fmt.Sprintf("\n    %d %s found:  %s\n\n",
		total, noun, strings.Join(parts, "  ")))

	sb.WriteString("    " + dimStyle.Render("Tip: Re-run with")+
		" "+valueStyle.Render("--html")+
		" "+dimStyle.Render("to view in browser, or")+
		" "+valueStyle.Render("--html-file-output report.html")+
		" "+dimStyle.Render("to save.")+"\n\n")

	return sb.String()
}

func countBySeverity(results []models.AIVulnerability) map[string]int {
	counts := make(map[string]int)
	for _, r := range results {
		counts[strings.ToLower(r.Severity)]++
	}
	return counts
}
