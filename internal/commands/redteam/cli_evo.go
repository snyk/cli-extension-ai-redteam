package redteam

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Block "EVO" art (UTF-8 box drawing); gradient applied per non-space rune.
var evoASCIILines = []string{
	"███████╗██╗   ██╗ ██████╗ ",
	"██╔════╝██║   ██║██╔═══██╗",
	"█████╗  ██║   ██║██║   ██║",
	"██╔══╝  ╚██╗ ██╔╝██║   ██║",
	"███████╗ ╚████╔╝ ╚██████╔╝",
	"╚══════╝  ╚═══╝   ╚═════╝ ",
}

func renderEVOLogo(theme *cliTheme) string {
	if theme.r.ColorProfile() == termenv.Ascii {
		return theme.logoFallback().Render(strings.Join(evoASCIILines, "\n"))
	}
	var b strings.Builder
	nonSpaceIdx := 0
	totalNonSpace := countNonSpace(evoASCIILines)
	if totalNonSpace == 0 {
		return ""
	}
	denom := totalNonSpace - 1
	if denom < 1 {
		denom = 1
	}
	for li, line := range evoASCIILines {
		for _, ch := range line {
			if ch == ' ' {
				b.WriteRune(ch)
				continue
			}
			t := float64(nonSpaceIdx) / float64(denom)
			st := gradientStyleAt(theme, t)
			b.WriteString(st.Render(string(ch)))
			nonSpaceIdx++
		}
		if li < len(evoASCIILines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func countNonSpace(lines []string) int {
	n := 0
	for _, line := range lines {
		for _, ch := range line {
			if ch != ' ' {
				n++
			}
		}
	}
	return n
}

// Interpolates purple (#A855F7) through pink (#EC4899) to orange (#F97316).
func gradientStyleAt(theme *cliTheme, t float64) lipgloss.Style {
	t = clampFloat(t, 0, 1)
	var r, g, b uint8
	switch {
	case t < 0.5:
		u := t * 2
		r = lerpByte(0xA8, 0xEC, u)
		g = lerpByte(0x55, 0x48, u)
		b = lerpByte(0xF7, 0x99, u)
	default:
		u := (t - 0.5) * 2
		r = lerpByte(0xEC, 0xF9, u)
		g = lerpByte(0x48, 0x73, u)
		b = lerpByte(0x99, 0x16, u)
	}
	hex := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
	return theme.r.NewStyle().Bold(true).Foreground(hex)
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func lerpByte(a, b uint8, t float64) uint8 {
	return uint8(float64(a)*(1-t) + float64(b)*t)
}
