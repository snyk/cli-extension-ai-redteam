package redteam

import (
	"fmt"
	"math"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
)

const (
	ansiCursorPrevLine = "\033[%dF"
	ansiEraseToEnd     = "\033[J"
)

// liveProgress renders the attack-strategies table in-place using ANSI
// cursor movement so that progress bars fill up without scrolling.
// On non-TTY outputs (CI, pipes) it skips live updates and renders once
// at the end via finish().
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
