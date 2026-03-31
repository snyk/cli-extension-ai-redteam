package redteam

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
)

// ---------------------------------------------------------------------------
// Live progress (in-place ANSI updates during the scan loop)
// ---------------------------------------------------------------------------

const (
	ansiCursorPrevLine = "\033[%dF"
	ansiEraseToEnd     = "\033[J"
)

// liveProgress renders the attacks table in-place using ANSI cursor movement
// so that progress bars fill up without scrolling. On non-TTY outputs (CI,
// pipes) it skips live updates and renders once at the end via finish().
type liveProgress struct {
	theme     *cliTheme
	width     int
	lineCount int
	lastFP    string
	isTTY     bool
}

func newLiveProgress(theme *cliTheme, width int) *liveProgress {
	fd := os.Stdout.Fd()
	tty := false
	if fd <= uintptr(math.MaxInt) {
		tty = term.IsTerminal(int(fd))
	}
	return &liveProgress{theme: theme, width: width, isTTY: tty}
}

// update re-renders the attack table in place when the status changes.
func (lp *liveProgress) update(status *controlserver.ScanStatus) {
	if status == nil || len(status.Attacks) == 0 {
		return
	}
	fp := statusFingerprint(status)
	if fp == lp.lastFP {
		return
	}
	lp.lastFP = fp

	if !lp.isTTY {
		return
	}

	block := lp.renderBlock(status, true)
	lp.overwrite(block)
}

// finish renders the final state of the table. For TTY it overwrites the
// live block; for non-TTY it prints the table once (without the progress
// indicator).
func (lp *liveProgress) finish(status *controlserver.ScanStatus) {
	if status == nil || len(status.Attacks) == 0 {
		return
	}
	block := lp.renderBlock(status, false)
	if lp.isTTY {
		lp.overwrite(block)
		lp.lineCount = 0
	} else {
		fmt.Fprint(os.Stdout, block)
	}
}

func (lp *liveProgress) overwrite(block string) {
	if lp.lineCount > 0 {
		fmt.Fprintf(os.Stdout, ansiCursorPrevLine+ansiEraseToEnd, lp.lineCount)
	}
	fmt.Fprint(os.Stdout, block)
	lp.lineCount = strings.Count(block, "\n")
}

func (lp *liveProgress) renderBlock(status *controlserver.ScanStatus, showProgress bool) string {
	var sb strings.Builder
	sb.WriteString(horizontalRule(lp.theme, "attacks", lp.width))
	sb.WriteString("\n")
	sb.WriteString("\n")
	for i := range status.Attacks {
		sb.WriteString(renderAttackRow(lp.theme, &status.Attacks[i], lp.width))
		sb.WriteString("\n")
	}
	if showProgress {
		pct := 0
		if status.TotalChats > 0 {
			pct = (status.Completed * 100) / status.TotalChats
		}
		fmt.Fprintf(&sb, "\n  %s\n",
			lp.theme.muted().Render(fmt.Sprintf("%d%% Scanning (%d/%d)", pct, status.Completed, status.TotalChats)))
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Status fingerprinting (dedup unchanged status updates)
// ---------------------------------------------------------------------------

func statusFingerprint(s *controlserver.ScanStatus) string {
	if s == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d/%d|%d|", s.Completed, s.TotalChats, len(s.Attacks))
	for _, a := range s.Attacks {
		fmt.Fprintf(&b, "%s:%d/%d/%d/%d;", a.AttackType, a.Completed, a.TotalChats, a.Failed, a.Successful)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Attack table rendering
// ---------------------------------------------------------------------------

func renderAttackStrategiesSection(theme *cliTheme, status *controlserver.ScanStatus, width int) string {
	if status == nil || len(status.Attacks) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(horizontalRule(theme, "attacks", width))
	sb.WriteString("\n\n")
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
	mark := rowStatusMark(theme, done, a.Successful > 0)
	label := truncateRunes(a.AttackType, 40)
	bar := renderProbeBar(theme, a.Completed, a.TotalChats, 10)

	findWord := "findings"
	if a.Successful == 1 {
		findWord = "finding"
	}
	findStyle := theme.success()
	if a.Successful > 0 {
		findStyle = theme.danger()
	}
	findingsStr := findStyle.Render(strconv.Itoa(a.Successful) + " " + findWord)
	probesStr := strconv.Itoa(a.TotalChats) + " probes"
	stats := findingsStr + " / " + probesStr

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

func rowStatusMark(theme *cliTheme, done, hasFindings bool) string {
	if theme.r.ColorProfile() == termenv.Ascii {
		switch {
		case hasFindings:
			return theme.danger().Render("!")
		case done:
			return theme.success().Render("v")
		default:
			return theme.muted().Render("-")
		}
	}
	switch {
	case hasFindings:
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

// ---------------------------------------------------------------------------
// Results summary (printed once after the scan completes)
// ---------------------------------------------------------------------------

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
