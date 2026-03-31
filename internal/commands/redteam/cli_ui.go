package redteam

import (
	"io"
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// ---------------------------------------------------------------------------
// Theme (color palette + style helpers)
// ---------------------------------------------------------------------------

// cliTheme groups lipgloss styles for red team CLI output.
type cliTheme struct {
	r *lipgloss.Renderer
}

// newCLITheme builds a renderer for stdout, honoring NO_COLOR and ASCII fallbacks.
func newCLITheme(out io.Writer) *cliTheme {
	if out == nil {
		out = os.Stdout
	}
	r := lipgloss.NewRenderer(out)
	if termenv.EnvNoColor() {
		r.SetColorProfile(termenv.Ascii)
	}
	return &cliTheme{r: r}
}

func (t *cliTheme) title() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("141")).Bold(true) // ansiPurple
}

func (t *cliTheme) subtitle() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("255")) // ansiWhite
}

func (t *cliTheme) muted() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("240")) // ansiDimGray
}

func (t *cliTheme) accent() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("141")) // ansiPurple
}

func (t *cliTheme) label() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("183")) // ansiLightPurple
}

func (t *cliTheme) success() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("114")) // ansiGreen
}

func (t *cliTheme) danger() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("203")) // ansiRed
}

func (t *cliTheme) logoFallback() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("183")).Bold(true) // ansiLightPurple
}

// ---------------------------------------------------------------------------
// Terminal layout helpers
// ---------------------------------------------------------------------------

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

// horizontalRule builds a centered label with em dash padding, e.g. "—— attacks ——".
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
