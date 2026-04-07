package redteam

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
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

// liveProgress renders the attacks table in-place using ANSI cursor movement.
// A background ticker redraws every 150 ms so spinners animate smoothly even
// while the main loop blocks on network calls. On non-TTY outputs (CI, pipes)
// it skips live updates and renders once at the end via finish().
type liveProgress struct {
	theme     *cliTheme
	width     int
	lineCount int
	isTTY     bool

	mu     sync.Mutex
	frame  int
	last   *controlserver.ScanStatus
	lastFP string
	done   chan struct{}
}

func newLiveProgress(theme *cliTheme, width int) *liveProgress {
	fd := os.Stdout.Fd()
	tty := false
	if fd <= uintptr(math.MaxInt) {
		tty = term.IsTerminal(int(fd))
	}
	lp := &liveProgress{theme: theme, width: width, isTTY: tty, done: make(chan struct{})}
	if tty {
		go lp.spin()
	}
	return lp
}

const spinInterval = 150 * time.Millisecond

// spin redraws the table on a timer so the waiting spinners animate.
func (lp *liveProgress) spin() {
	t := time.NewTicker(spinInterval)
	defer t.Stop()
	for {
		select {
		case <-lp.done:
			return
		case <-t.C:
			lp.mu.Lock()
			if lp.last != nil {
				lp.frame++
				block := lp.renderBlock(lp.last, true)
				lp.overwrite(block)
			}
			lp.mu.Unlock()
		}
	}
}

// update stores the latest status and triggers a redraw. On non-TTY it
// only records the status for the final finish() call.
func (lp *liveProgress) update(status *controlserver.ScanStatus) {
	if status == nil || len(status.Attacks) == 0 {
		return
	}
	lp.mu.Lock()
	defer lp.mu.Unlock()

	lp.last = status
	fp := statusFingerprint(status)
	lp.lastFP = fp

	if lp.isTTY {
		lp.frame++
		block := lp.renderBlock(status, true)
		lp.overwrite(block)
	}
}

// finish renders the final state of the table and stops the spinner goroutine.
func (lp *liveProgress) finish(status *controlserver.ScanStatus) {
	close(lp.done)
	if status == nil || len(status.Attacks) == 0 {
		return
	}
	lp.mu.Lock()
	defer lp.mu.Unlock()

	lp.last = status
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
	activeIdx := firstNonDoneIdx(status.Attacks)
	for i := range status.Attacks {
		sb.WriteString(renderAttackRow(lp.theme, &status.Attacks[i], lp.frame, i == activeIdx))
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
	activeIdx := firstNonDoneIdx(status.Attacks)
	for i := range status.Attacks {
		sb.WriteString(renderAttackRow(theme, &status.Attacks[i], 0, i == activeIdx))
		sb.WriteString("\n")
	}
	return sb.String()
}

func firstNonDoneIdx(attacks []controlserver.AttackStatus) int {
	for i, a := range attacks {
		if a.TotalChats == 0 || a.Completed < a.TotalChats {
			return i
		}
	}
	return -1
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func renderAttackRow(theme *cliTheme, a *controlserver.AttackStatus, frame int, isActive bool) string {
	if a == nil {
		return ""
	}
	done := a.TotalChats > 0 && a.Completed >= a.TotalChats
	mark := rowStatusMark(theme, done, isActive && !done, a.Successful > 0, frame)
	label := truncateRunes(humanizeAttackType(a.AttackType), 50)

	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(mark)
	sb.WriteString(" ")

	if !done && !isActive {
		sb.WriteString(theme.muted().Render(label))
		return sb.String()
	}

	sb.WriteString(theme.subtitle().Render(label))
	sb.WriteString("  ")
	sb.WriteString(attackRowStats(theme, a, done, isActive))
	return sb.String()
}

func attackRowStats(theme *cliTheme, a *controlserver.AttackStatus, done, isActive bool) string {
	if isActive && !done {
		return theme.muted().Render(strconv.Itoa(a.Completed) + "/" + strconv.Itoa(a.TotalChats))
	}
	findWord := "findings"
	if a.Successful == 1 {
		findWord = "finding"
	}
	findingsText := strconv.Itoa(a.Successful) + " " + findWord
	if a.Successful > 0 {
		findingsText = theme.danger().Render(findingsText)
	} else {
		findingsText = theme.success().Render(findingsText)
	}
	return strconv.Itoa(a.TotalChats) + " probes / " + findingsText
}

func rowStatusMark(theme *cliTheme, done, inProgress, hasFindings bool, frame int) string {
	ascii := theme.r.ColorProfile() == termenv.Ascii
	switch {
	case done && hasFindings:
		if ascii {
			return theme.danger().Render("!")
		}
		return theme.danger().Render("✗")
	case done:
		if ascii {
			return theme.success().Render("v")
		}
		return theme.success().Render("✓")
	case inProgress:
		if ascii {
			return theme.accent().Render("*")
		}
		return theme.accent().Render("●")
	default:
		if ascii {
			return theme.muted().Render(string("/-\\|"[frame%4]))
		}
		return theme.muted().Render(spinnerFrames[frame%len(spinnerFrames)])
	}
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

func humanizeAttackType(s string) string {
	parts := strings.Split(s, "/")
	if len(parts) > 0 {
		if _, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			parts = parts[:len(parts)-1]
		}
	}
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(p, "_", " ")
	}
	return strings.Join(parts, " / ")
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
