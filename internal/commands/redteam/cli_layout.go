package redteam

import (
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const defaultTermWidth = 80

func terminalWidth() int {
	fd := os.Stdout.Fd()
	if fd > uintptr(math.MaxInt) {
		return defaultTermWidth
	}
	w, _, err := term.GetSize(int(fd))
	if err != nil || w < 40 {
		return defaultTermWidth
	}
	if w > 120 {
		return 120
	}
	return w
}

// horizontalRule builds a centered label with em dash padding, e.g. "—— scan configuration ——".
func horizontalRule(theme *cliTheme, label string, width int) string {
	label = strings.TrimSpace(label)
	pad := strings.Repeat("—", 2)
	inner := pad + " " + label + " " + pad
	if lipgloss.Width(inner) >= width {
		return theme.muted().Render(inner)
	}
	remain := width - lipgloss.Width(inner)
	left := remain / 2
	right := remain - left
	line := strings.Repeat("—", left) + inner + strings.Repeat("—", right)
	return theme.muted().Render(line)
}
