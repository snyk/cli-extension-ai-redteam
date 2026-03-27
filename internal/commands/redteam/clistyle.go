package redteam

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// cliTheme groups lipgloss styles for red team CLI output (mockup palette).
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
	return t.r.NewStyle().Foreground(lipgloss.Color("#B794F6")).Bold(true)
}

func (t *cliTheme) subtitle() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
}

func (t *cliTheme) muted() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
}

func (t *cliTheme) accent() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("#C084FC"))
}

func (t *cliTheme) label() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("#A78BFA"))
}

func (t *cliTheme) success() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("#4ADE80"))
}

func (t *cliTheme) danger() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("#F87171"))
}

func (t *cliTheme) tag() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("#C084FC"))
}

func (t *cliTheme) logoFallback() lipgloss.Style {
	return t.r.NewStyle().Foreground(lipgloss.Color("#A855F7")).Bold(true)
}
