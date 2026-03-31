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
