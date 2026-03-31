package redteam

import (
	"fmt"
	"strings"
)

var evoLogoLines = []string{
	" ███████╗ ██╗   ██╗  ██████╗ ",
	" ██╔════╝ ██║   ██║ ██╔═══██╗",
	" █████╗   ██║   ██║ ██║   ██║",
	" ██╔══╝   ╚██╗ ██╔╝ ██║   ██║",
	" ███████╗  ╚████╔╝  ╚██████╔╝",
}

const evoLogoLastLine = " ╚══════╝   ╚═══╝    ╚═════╝ "

func renderEVOLogo(theme *cliTheme) string {
	purple := theme.logoFallback()
	white := theme.subtitle()
	var sb strings.Builder
	for _, line := range evoLogoLines {
		fmt.Fprintf(&sb, "  %s\n", purple.Render(line))
	}
	fmt.Fprintf(&sb, "  %s  %s", purple.Render(evoLogoLastLine), white.Render("by Snyk"))
	return sb.String()
}
