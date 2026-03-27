package redteam

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/muesli/termenv"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
)

func statusFingerprint(s *controlserver.ScanStatus) string {
	if s == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d/%d|%d|", s.Completed, s.TotalChats, len(s.Attacks))
	for _, a := range s.Attacks {
		fmt.Fprintf(&b, "%s:%d/%d/%d;", a.AttackType, a.Completed, a.TotalChats, a.Failed)
	}
	return b.String()
}

func renderAttackStrategiesSection(theme *cliTheme, status *controlserver.ScanStatus, width int) string {
	if status == nil || len(status.Attacks) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(horizontalRule(theme, "attack strategies", width))
	sb.WriteString("\n")
	if len(status.Tags) > 0 {
		tagStr := "[" + strings.Join(status.Tags, ", ") + "]"
		sb.WriteString("  ")
		sb.WriteString(theme.tag().Render(tagStr))
		sb.WriteString("\n\n")
	}
	for i := range status.Attacks {
		sb.WriteString(renderAttackRow(theme, &status.Attacks[i], width))
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderAttackRow(theme *cliTheme, a *controlserver.AttackStatus, _ int) string {
	if a == nil {
		return ""
	}
	done := a.TotalChats > 0 && a.Completed >= a.TotalChats
	mark := rowStatusMark(theme, done, a.Failed > 0)
	label := truncateRunes(a.AttackType, 40)
	bar := renderProbeBar(theme, a.Completed, a.TotalChats, 10)
	probeLine := theme.success().Render(strconv.Itoa(a.Completed)) + "/" + theme.success().Render(strconv.Itoa(a.TotalChats)) + " probes"
	stats := theme.muted().Render(" — ") + probeLine
	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(mark)
	sb.WriteString(" ")
	sb.WriteString(theme.subtitle().Render(label))
	sb.WriteString("  ")
	sb.WriteString(bar)
	sb.WriteString("  ")
	sb.WriteString(stats)
	return sb.String()
}

func rowStatusMark(theme *cliTheme, done, hasFailed bool) string {
	if theme.r.ColorProfile() == termenv.Ascii {
		switch {
		case hasFailed:
			return theme.danger().Render("!")
		case done:
			return theme.success().Render("v")
		default:
			return theme.muted().Render("-")
		}
	}
	switch {
	case hasFailed:
		return theme.danger().Render("✗")
	case done:
		return theme.success().Render("✓")
	default:
		return theme.muted().Render("·")
	}
}

func renderProbeBar(theme *cliTheme, completed, total, barW int) string {
	if total <= 0 {
		return theme.muted().Render(strings.Repeat(blockEmpty(theme), barW))
	}
	filled := (completed * barW) / total
	if completed > 0 && filled == 0 {
		filled = 1
	}
	if filled > barW {
		filled = barW
	}
	f := blockFilled(theme)
	e := blockEmpty(theme)
	return theme.success().Render(strings.Repeat(f, filled)) + theme.muted().Render(strings.Repeat(e, barW-filled))
}

func blockFilled(theme *cliTheme) string {
	if theme.r.ColorProfile() == termenv.Ascii {
		return "#"
	}
	return "█"
}

func blockEmpty(theme *cliTheme) string {
	if theme.r.ColorProfile() == termenv.Ascii {
		return "."
	}
	return "░"
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= maxRunes {
		return s
	}
	return string(rs[:maxRunes-1]) + "…"
}

func reportFindingsCount(reportJSON []byte) int {
	var v struct {
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(reportJSON, &v); err != nil {
		return 0
	}
	return len(v.Results)
}

func renderResultsSummaryLine(theme *cliTheme, findings, probes, strategies int, elapsed time.Duration) string {
	sec := int(elapsed.Round(time.Second) / time.Second)
	if sec < 0 {
		sec = 0
	}
	findWord := "findings"
	if findings == 1 {
		findWord = "finding"
	}
	parts := []string{
		theme.danger().Render(strconv.Itoa(findings)) + theme.subtitle().Render(" "+findWord),
		theme.subtitle().Render(pluralUnit(probes, "probe", "probes")),
		theme.subtitle().Render(pluralUnit(strategies, "strategy", "strategies")),
		theme.subtitle().Render(strconv.Itoa(sec) + "s elapsed"),
	}
	line := "\n" + horizontalRule(theme, "results", terminalWidth()) + "\n  "
	line += strings.Join(parts, " · ")
	line += "\n"
	return line
}

func pluralUnit(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}
