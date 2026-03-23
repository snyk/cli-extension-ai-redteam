package clireport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/snyk/cli-extension-ai-redteam/internal/models"
)

const (
	maxEvidenceLen   = 300
	maxConvoLen      = 300
	defaultTermWidth = 120
	colGap           = 3
	indentFmt        = "      %s\n"
)

// layout holds responsive widths computed from the terminal width.
type layout struct {
	termWidth        int
	content          int // usable width inside report box (border + padding removed)
	strategy         int
	severity         int
	blocked          int
	findingCandidate int
	tableWidth       int
	chatBox          int
}

func newLayout(termWidth int) layout {
	if termWidth <= 0 {
		termWidth = defaultTermWidth
	}
	// Report box eats ~6 chars (border + padding on each side).
	content := termWidth - 6
	if content < 60 {
		content = 60
	}

	// Chat/evidence boxes: content minus left margin (6) and box border/padding (4).
	chatBox := content - 10
	if chatBox < 40 {
		chatBox = 40
	}

	// Table columns: distribute proportionally within content width.
	// Reserve indent (4) + 3 gaps (3*3=9) = 13 chars.
	available := content - 13
	if available < 40 {
		available = 40
	}

	// Proportions: strategy 45%, severity 15%, blocked 20%, finding candidate 20%
	strategy := available * 45 / 100
	severity := available * 15 / 100
	blocked := available * 20 / 100
	findingCandidate := available - strategy - severity - blocked

	tw := strategy + severity + blocked + findingCandidate + (3 * colGap)

	return layout{
		termWidth:        termWidth,
		content:          content,
		strategy:         strategy,
		severity:         severity,
		blocked:          blocked,
		findingCandidate: findingCandidate,
		tableWidth:       tw,
		chatBox:          chatBox,
	}
}

// ScanMeta holds metadata about the scan for display in the report header.
type ScanMeta struct {
	TargetURL        string
	Goals            []string
	Strategies       []string
	FullConversation bool
}

// Render produces a styled CLI report from normalized scan results.
func Render(data *models.GetAIVulnerabilitiesResponseData, meta ScanMeta) string {
	return RenderWithWidth(data, meta, defaultTermWidth)
}

// RenderWithWidth produces a styled CLI report responsive to the given terminal width.
func RenderWithWidth(data *models.GetAIVulnerabilitiesResponseData, meta ScanMeta, termWidth int) string {
	l := newLayout(termWidth)
	var sb strings.Builder

	sb.WriteString(renderBanner(data))
	sb.WriteString(renderHeader(meta))
	sb.WriteString(renderSummary(data))

	if data.Summary != nil && len(data.Summary.Vulnerabilities) > 0 {
		sb.WriteString(renderStrategyTable(data.Summary, l))
	}

	if len(data.Results) > 0 {
		sb.WriteString(renderFindings(data.Results, data.Summary, meta.FullConversation, l))
	}

	if len(data.PassedTypes) > 0 {
		sb.WriteString(renderPassedTypes(data.PassedTypes))
	}

	sb.WriteString(renderFooter(data))

	box := reportBoxStyle.Width(l.termWidth - 2)
	return box.Render(sb.String()) + "\n"
}

// --- palette ---

var (
	colPurple   = lipgloss.Color("#CBABEE") // headlines + agent text
	colRed      = lipgloss.Color("#E44A50") // all red: fail, high sev, etc.
	colMedSev   = lipgloss.Color("#b7950b")
	colLowSev   = lipgloss.Color("#1a5276")
	colPass     = lipgloss.Color("#7BF1A8")
	colSlate    = lipgloss.Color("#5d6d7e")
	colDim      = lipgloss.Color("#7f8c8d")
	colWhite    = lipgloss.Color("#FFFFFF")
	colBlack    = lipgloss.Color("#000000")
	colDarkGray = lipgloss.Color("#2d2d2d")

	headingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colPurple)

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

	sevBadgeHigh = lipgloss.NewStyle().
			Bold(true).
			Foreground(colDarkGray).
			Background(colRed)

	sevBadgeMed = lipgloss.NewStyle().
			Bold(true).
			Foreground(colBlack).
			Background(colMedSev)

	sevBadgeLow = lipgloss.NewStyle().
			Bold(true).
			Foreground(colWhite).
			Background(colLowSev)

	findingCandidateBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(colDarkGray).
				Background(colRed)

	bannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colDarkGray).
			Background(colRed).
			Padding(0, 1)

	bannerCleanStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colDarkGray).
				Background(colPass).
				Padding(0, 1)

	reportBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colSlate).
			Padding(1, 2)

	userBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colSlate).
			Padding(0, 1).
			MarginLeft(6).
			Width(140)

	agentBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colPurple).
			Padding(0, 1).
			MarginLeft(6).
			Width(140)

	evidenceBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colSlate).
				Padding(0, 1).
				MarginLeft(6).
				Width(140)
)

// --- banner ---

//nolint:nestif // sequential branching for banner rendering
func renderBanner(data *models.GetAIVulnerabilitiesResponseData) string {
	var sb strings.Builder

	findingCandidates, blocked, totalProbes := countOutcomes(data)

	sb.WriteString("\n")
	if findingCandidates == 0 {
		banner := fmt.Sprintf(" \u2713 All %d probes blocked \u2014 no vulnerabilities found ", totalProbes)
		sb.WriteString("  " + bannerCleanStyle.Render(banner))
	} else {
		counts := countBySeverity(data.Results)
		var parts []string
		if n, ok := counts["critical"]; ok && n > 0 {
			parts = append(parts, fmt.Sprintf("%d CRITICAL", n))
		}
		if n, ok := counts["high"]; ok && n > 0 {
			parts = append(parts, fmt.Sprintf("%d HIGH", n))
		}
		if n, ok := counts["medium"]; ok && n > 0 {
			parts = append(parts, fmt.Sprintf("%d MEDIUM", n))
		}
		if n, ok := counts["low"]; ok && n > 0 {
			parts = append(parts, fmt.Sprintf("%d LOW", n))
		}

		noun := "FINDING CANDIDATE"
		if findingCandidates > 1 {
			noun = "FINDING CANDIDATES"
		}
		banner := fmt.Sprintf(" \u26a0 %d %s   %d blocked   (%d probes) ",
			findingCandidates, noun, blocked, totalProbes)
		if len(parts) > 0 {
			banner = fmt.Sprintf(" \u26a0 %d %s: %s   %d blocked   (%d probes) ",
				findingCandidates, noun, strings.Join(parts, ", "), blocked, totalProbes)
		}
		sb.WriteString("  " + bannerStyle.Render(banner))
	}
	sb.WriteString("\n\n")

	return sb.String()
}

func countOutcomes(data *models.GetAIVulnerabilitiesResponseData) (findingCandidates, blocked, total int) {
	if data.Summary == nil {
		return 0, 0, 0
	}
	for i := range data.Summary.Vulnerabilities {
		v := &data.Summary.Vulnerabilities[i]
		total += v.TotalChats
		findingCandidates += v.Successful
		blocked += v.TotalChats - v.Successful
	}
	return findingCandidates, blocked, total
}

// --- header ---

func renderHeader(meta ScanMeta) string {
	var sb strings.Builder

	sb.WriteString("  " + headingStyle.Render("Scan Metadata"))
	sb.WriteString("\n\n")
	sb.WriteString("    " + labelStyle.Render("Target:     ") + valueStyle.Render(meta.TargetURL) + "\n")
	sb.WriteString("    " + labelStyle.Render("Goals:      ") + valueStyle.Render(strings.Join(meta.Goals, ", ")) + "\n")
	sb.WriteString("    " + labelStyle.Render("Strategies: ") + valueStyle.Render(strings.Join(meta.Strategies, ", ")) + "\n")
	sb.WriteString("\n")

	return sb.String()
}

// --- summary ---

func renderSummary(data *models.GetAIVulnerabilitiesResponseData) string {
	var vulnerable, blocked, skipped int

	if data.Summary != nil {
		for i := range data.Summary.Vulnerabilities {
			v := &data.Summary.Vulnerabilities[i]
			switch {
			case v.Vulnerable:
				vulnerable++
			case v.Status == "completed":
				blocked++
			default:
				skipped++
			}
		}
	}

	var sb strings.Builder

	sb.WriteString("  " + headingStyle.Render("Summary"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "    %s %d vulnerable    %s %d blocked    %s %d skipped\n\n",
		failText.Render("\u26a0"), vulnerable,
		passText.Render("\u2713"), blocked,
		dimStyle.Render("\u25cc"), skipped,
	)

	return sb.String()
}

// --- strategy table ---

func renderStrategyTable(summary *models.AIScanSummary, l layout) string {
	var sb strings.Builder
	gap := strings.Repeat(" ", colGap)

	sb.WriteString("  " + headingStyle.Render("Strategy Breakdown"))
	sb.WriteString("\n\n")

	dash := dimStyle.Render(strings.Repeat("\u2500", l.tableWidth))

	sb.WriteString("    " + dash + "\n")

	fmt.Fprintf(&sb, "    %s%s%s%s%s%s%s\n",
		padRight(dimStyle.Render("STRATEGY"), l.strategy),
		gap,
		padRight(dimStyle.Render("SEVERITY"), l.severity),
		gap,
		padRight(dimStyle.Render("BLOCKED"), l.blocked),
		gap,
		padRight(dimStyle.Render("FINDING CANDIDATE"), l.findingCandidate),
	)
	sb.WriteString("    " + dash + "\n")

	for i := range summary.Vulnerabilities {
		v := &summary.Vulnerabilities[i]
		leaf := truncateCol(leafStrategy(v.EngineTag), l.strategy)
		sev := padRight(renderSeverityText(v.Severity), l.severity)
		blocked := v.TotalChats - v.Successful
		candidates := v.Successful

		blockedStr := padRight(passText.Render(fmt.Sprintf("%d", blocked)), l.blocked)
		var candidateStr string
		if candidates > 0 {
			candidateStr = padRight(failText.Render(fmt.Sprintf("\u25cf %d", candidates)), l.findingCandidate)
		} else {
			candidateStr = padRight(dimStyle.Render("0"), l.findingCandidate)
		}

		fmt.Fprintf(&sb, "    %s%s%s%s%s%s%s\n",
			padRight(valueStyle.Render(leaf), l.strategy),
			gap, sev,
			gap, blockedStr,
			gap, candidateStr,
		)
		fmt.Fprintf(&sb, indentFmt, dimStyle.Render(v.EngineTag))
	}

	sb.WriteString("    " + dash + "\n\n\n")
	return sb.String()
}

// padRight pads a styled string to a fixed visible width.
// ANSI escape codes are excluded from the width count.
func padRight(styled string, width int) string {
	visible := stripAnsi(styled)
	pad := width - len(visible)
	if pad <= 0 {
		return styled
	}
	return styled + strings.Repeat(" ", pad)
}

func stripAnsi(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func leafStrategy(engineTag string) string {
	parts := strings.Split(engineTag, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return engineTag
}

func truncateCol(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-1] + "\u2026"
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

func renderSeverityBadge(sev string) string {
	upper := strings.ToUpper(sev)
	label := fmt.Sprintf(" \u26a0 %s ", upper)
	switch strings.ToLower(sev) {
	case "critical", "high":
		return sevBadgeHigh.Render(label)
	case "medium":
		return sevBadgeMed.Render(label)
	case "low":
		return sevBadgeLow.Render(label)
	default:
		return sevBadgeMed.Render(label)
	}
}

// owaspLabel returns a human-readable label for known OWASP LLM Top 10 tags.
func owaspLabel(tags []string) string {
	lookup := map[string]string{
		"LLM01": "LLM01: Prompt Injection",
		"LLM02": "LLM02: Insecure Output Handling",
		"LLM03": "LLM03: Training Data Poisoning",
		"LLM04": "LLM04: Model Denial of Service",
		"LLM05": "LLM05: Supply Chain Vulnerabilities",
		"LLM06": "LLM06: Sensitive Information Disclosure",
		"LLM07": "LLM07: Insecure Plugin Design",
		"LLM08": "LLM08: Excessive Agency",
		"LLM09": "LLM09: Overreliance",
		"LLM10": "LLM10: Model Theft",
	}
	var labels []string
	for _, tag := range tags {
		upper := strings.ToUpper(tag)
		for prefix, label := range lookup {
			if strings.Contains(upper, prefix) {
				labels = append(labels, label)
				break
			}
		}
	}
	if len(labels) > 0 {
		return strings.Join(labels, ", ")
	}
	return ""
}

// --- findings ---

func renderFindings(results []models.AIVulnerability, summary *models.AIScanSummary, fullConversation bool, l layout) string {
	var sb strings.Builder

	sb.WriteString("  " + headingStyle.Render("Findings"))
	sb.WriteString("\n")

	for i := range results {
		vuln := &results[i]
		sb.WriteString("\n")

		// Finding header with severity + finding candidate badges inline.
		fmt.Fprintf(&sb, "    %s  %s  %s  %s\n",
			failText.Render(fmt.Sprintf("\u25bc #%d", i+1)),
			valueStyle.Render(vuln.Definition.Name),
			renderSeverityBadge(vuln.Severity),
			findingCandidateBadge.Render(" FINDING CANDIDATE "),
		)
		fmt.Fprintf(&sb, "    %s\n", dimStyle.Render(vuln.Definition.Description))

		// Probe result.
		total := findingPassRate(vuln.Definition.ID, summary)
		candidateCount := findingFailCount(vuln.Definition.ID, summary)
		fmt.Fprintf(&sb, "    %s\n",
			failText.Render(fmt.Sprintf("%d of %d probes are finding candidates", candidateCount, total)),
		)

		// OWASP reference inline.
		if len(vuln.Tags) > 0 {
			owasp := owaspLabel(vuln.Tags)
			if owasp != "" {
				fmt.Fprintf(&sb, "    %s\n", valueStyle.Render(owasp))
			} else {
				fmt.Fprintf(&sb, "    %s %s\n",
					labelStyle.Render("Tested Vulnerabilities:"),
					dimStyle.Render(strings.Join(vuln.Tags, ", ")),
				)
			}
		}

		if fullConversation {
			sb.WriteString("\n")
			sb.WriteString(renderConversation(vuln.Turns, true, l))

			if vuln.Evidence.Content.Reason != "" {
				renderEvidenceBlock(&sb, vuln.Evidence.Content.Reason, l)
			}
		} else {
			fmt.Fprintf(&sb, "    %s\n",
				dimStyle.Render("[use --full-conversation to expand chat details]"))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

func renderEvidenceBlock(sb *strings.Builder, reason string, l layout) {
	fullLen := len(reason)
	evidence := reason
	truncated := false
	if fullLen > maxEvidenceLen {
		evidence = reason[:maxEvidenceLen]
		truncated = true
	}

	header := failText.Render("Extracted content (finding candidate)")
	content := fmt.Sprintf("%s\n%s", header, dimStyle.Render(evidence))
	if truncated {
		content += fmt.Sprintf("\n%s",
			labelStyle.Render(fmt.Sprintf("Showing %d / %d chars", maxEvidenceLen, fullLen)))
	}
	sb.WriteString(evidenceBoxStyle.Width(l.chatBox).Render(content))
	sb.WriteString("\n")
}

func findingPassRate(defID string, summary *models.AIScanSummary) (total int) {
	if summary == nil {
		return 1
	}
	for i := range summary.Vulnerabilities {
		v := &summary.Vulnerabilities[i]
		if v.Slug == defID {
			return v.TotalChats
		}
	}
	return 1
}

func findingFailCount(defID string, summary *models.AIScanSummary) int {
	if summary == nil {
		return 0
	}
	for i := range summary.Vulnerabilities {
		if summary.Vulnerabilities[i].Slug == defID {
			return summary.Vulnerabilities[i].Successful
		}
	}
	return 0
}

// renderConversation renders turns as a chat-style layout.
// By default only the first and last turns are shown. Pass fullConversation=true
// to display all turns (--full-conversation flag).
func renderConversation(turns []models.Turn, fullConversation bool, l layout) string {
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
			sb.WriteString(userBoxStyle.Width(l.chatBox).Render(content))
			sb.WriteString("\n")
		}
		if turn.Response != nil {
			msg := truncate(*turn.Response, maxConvoLen)
			content := fmt.Sprintf("%s  %s", purpleText.Render("Agent"), valueStyle.Render(msg))
			sb.WriteString(agentBoxStyle.Width(l.chatBox).Render(content))
			sb.WriteString("\n")
		}

		if i == 0 && omitted > 0 {
			noun := "turn"
			if omitted > 1 {
				noun = "turns"
			}
			fmt.Fprintf(&sb, indentFmt,
				dimStyle.Render(fmt.Sprintf("... %d %s hidden (use --full-conversation to expand)", omitted, noun)))
		}
	}

	return sb.String()
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "\u2026"
	}
	return s
}

// --- passed types ---

func renderPassedTypes(passedTypes []models.PassedType) string {
	var sb strings.Builder

	sb.WriteString("  " + headingStyle.Render("Passed Types"))
	sb.WriteString("\n\n")

	for i := range passedTypes {
		pt := &passedTypes[i]
		fmt.Fprintf(&sb, "    %s  %s\n",
			passText.Render("\u2713"),
			valueStyle.Render(pt.Name),
		)
		fmt.Fprintf(&sb, indentFmt, dimStyle.Render(pt.ID))
	}

	sb.WriteString("\n")
	return sb.String()
}

// --- footer ---

func renderFooter(data *models.GetAIVulnerabilitiesResponseData) string {
	var sb strings.Builder

	if len(data.Results) == 0 {
		sb.WriteString("\n")
		return sb.String()
	}

	sb.WriteString("\n    " + dimStyle.Render("Tip: Re-run with") +
		" " + valueStyle.Render("--html") +
		" " + dimStyle.Render("to view in browser, or") +
		" " + valueStyle.Render("--html-file-output report.html") +
		" " + dimStyle.Render("to save.") + "\n")
	evoLink := "\033]8;;https://evo.snyk.io/report\033\\https://evo.snyk.io/report\033]8;;\033\\"
	sb.WriteString("    " + dimStyle.Render("View full report at") +
		" " + evoLink + "\n\n")

	return sb.String()
}

func countBySeverity(results []models.AIVulnerability) map[string]int {
	counts := make(map[string]int)
	for i := range results {
		counts[strings.ToLower(results[i].Severity)]++
	}
	return counts
}

// --- report persistence ---

// SavedReport bundles scan data and metadata for re-display.
type SavedReport struct {
	Data *models.GetAIVulnerabilitiesResponseData `json:"data"`
	Meta ScanMeta                                 `json:"meta"`
}

const reportFileName = "redteam-last-report.json"

func reportPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	dir := filepath.Join(home, ".snyk")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create .snyk directory: %w", err)
	}
	return filepath.Join(dir, reportFileName), nil
}

// SaveReport persists the latest scan report so it can be re-opened with --report.
func SaveReport(data *models.GetAIVulnerabilitiesResponseData, meta ScanMeta) error {
	p, err := reportPath()
	if err != nil {
		return err
	}
	b, err := json.Marshal(SavedReport{Data: data, Meta: meta})
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}
	return os.WriteFile(p, b, 0o600) //nolint:wrapcheck // WriteFile error is self-explanatory
}

// LoadReport loads the last saved report from disk.
func LoadReport() (*models.GetAIVulnerabilitiesResponseData, ScanMeta, error) {
	p, err := reportPath()
	if err != nil {
		return nil, ScanMeta{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ScanMeta{}, fmt.Errorf("no saved report found — run a scan first")
		}
		return nil, ScanMeta{}, fmt.Errorf("failed to read report file: %w", err)
	}
	var saved SavedReport
	if err := json.Unmarshal(b, &saved); err != nil {
		return nil, ScanMeta{}, fmt.Errorf("saved report is corrupted: %w", err)
	}
	return saved.Data, saved.Meta, nil
}
