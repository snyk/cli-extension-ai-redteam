package redteam

import (
	"fmt"
	"os"
	"strings"

	"github.com/snyk/go-application-framework/pkg/ui"
	"golang.org/x/term"
)

// bannerParams holds the values displayed in the startup banner.
type bannerParams struct {
	ScanID      string
	TargetURL   string
	ProfileName string
	Goals       []string
	Strategies  []string
	ConfigPath  string
}

// ANSI color/style constants.
const (
	ansiReset    = "\033[0m"
	ansiBold     = "\033[1m"
	ansiPurple      = "\033[38;5;141m"
	ansiLightPurple = "\033[38;5;183m"
	ansiDimGray  = "\033[38;5;240m"
	ansiVeryDark = "\033[38;5;236m"
	ansiBlu      = "\033[38;5;111m"
	ansiGreen    = "\033[38;5;114m"
	ansiRed      = "\033[38;5;203m"
	ansiOrange   = "\033[38;5;208m"
	ansiBgPurple = "\033[48;5;141m"
	ansiWhite    = "\033[38;5;255m"
	ansiClearLn  = "\033[K"
)

// supportsColor returns true when ANSI color output is appropriate.
func supportsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// printLogo renders "evo" in a small ASCII art with "by Snyk" beside it.
// Skipped entirely in non-color / non-TTY mode.
func printLogo(userInterface ui.UserInterface) {
	if !supportsColor() {
		return
	}

	lp := ansiLightPurple
	w := ansiWhite
	r := ansiReset

	var sb strings.Builder
	fmt.Fprintf(&sb, "\n  %s ███████╗ ██╗   ██╗  ██████╗ %s\n", lp, r)
	fmt.Fprintf(&sb, "  %s ██╔════╝ ██║   ██║ ██╔═══██╗%s\n", lp, r)
	fmt.Fprintf(&sb, "  %s █████╗   ██║   ██║ ██║   ██║%s\n", lp, r)
	fmt.Fprintf(&sb, "  %s ██╔══╝   ╚██╗ ██╔╝ ██║   ██║%s\n", lp, r)
	fmt.Fprintf(&sb, "  %s ███████╗  ╚████╔╝  ╚██████╔╝%s\n", lp, r)
	fmt.Fprintf(&sb, "  %s ╚══════╝   ╚═══╝    ╚═════╝ %s  %sby Snyk%s\n\n", lp, r, w, r)

	_ = userInterface.Output(sb.String()) //nolint:errcheck // best-effort logo output
}

// printBanner renders the static header section (config table + strategy pills).
func printBanner(userInterface ui.UserInterface, p bannerParams) {
	var out string
	if supportsColor() {
		out = renderColorBanner(p)
	} else {
		out = renderPlainBanner(p)
	}
	_ = userInterface.Output(out) //nolint:errcheck // best-effort banner output
}

func renderColorBanner(p bannerParams) string {
	var sb strings.Builder

	// Title.
	fmt.Fprintf(&sb, "  %s%sAI Red Teaming%s\n",
		ansiBold, ansiLightPurple, ansiReset)

	// Subtitle.
	fmt.Fprintf(&sb, "  %sAdversarial testing for AI-native applications%s\n", ansiWhite, ansiReset)
	sb.WriteString("\n")

	// Session ID.
	if p.ScanID != "" {
		fmt.Fprintf(&sb, "  %ssession %s%s\n\n", ansiWhite, p.ScanID, ansiReset)
	}

	// Scan configuration header.
	fmt.Fprintf(&sb, "  %s\u2500\u2500\u2500 scan configuration \u2500\u2500\u2500%s\n\n", ansiWhite, ansiReset)

	// Config table.
	fmt.Fprintf(&sb, "  %s%-10s%s %s%s%s\n", ansiWhite, "Target", ansiReset, ansiPurple, p.TargetURL, ansiReset)
	fmt.Fprintf(&sb, "  %s%-10s%s %s%s%s\n", ansiWhite, "Goal", ansiReset, ansiWhite, strings.Join(p.Goals, ", "), ansiReset)

	mode := "profile"
	if p.ProfileName == "" {
		mode = "custom"
	}
	fmt.Fprintf(&sb, "  %s%-10s%s %s%s%s\n", ansiWhite, "Mode", ansiReset, ansiWhite, mode, ansiReset)

	cfgDisplay := p.ConfigPath
	if cfgDisplay == "" {
		cfgDisplay = "flags"
	}
	fmt.Fprintf(&sb, "  %s%-10s%s %s%s%s\n", ansiWhite, "Config", ansiReset, ansiWhite, cfgDisplay, ansiReset)
	sb.WriteString("\n")

	// Strategy pills.
	if len(p.Strategies) > 0 {
		sb.WriteString("  ")
		for i, s := range p.Strategies {
			if i == 0 {
				fmt.Fprintf(&sb, "%s[%s]%s", ansiLightPurple, s, ansiReset)
			} else {
				fmt.Fprintf(&sb, " %s(%s)%s", ansiDimGray, s, ansiReset)
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	return sb.String()
}

func renderPlainBanner(p bannerParams) string {
	var sb strings.Builder

	sb.WriteString("  AI Red Teaming\n")
	sb.WriteString("  Adversarial testing for AI-native applications\n\n")

	if p.ScanID != "" {
		fmt.Fprintf(&sb, "  session %s\n\n", p.ScanID)
	}

	sb.WriteString("  --- scan configuration ---\n\n")

	fmt.Fprintf(&sb, "  %-10s %s\n", "Target", p.TargetURL)
	fmt.Fprintf(&sb, "  %-10s %s\n", "Goal", strings.Join(p.Goals, ", "))

	mode := "profile"
	if p.ProfileName == "" {
		mode = "custom"
	}
	fmt.Fprintf(&sb, "  %-10s %s\n", "Mode", mode)

	cfgDisplay := p.ConfigPath
	if cfgDisplay == "" {
		cfgDisplay = "flags"
	}
	fmt.Fprintf(&sb, "  %-10s %s\n", "Config", cfgDisplay)
	sb.WriteString("\n")

	if len(p.Strategies) > 0 {
		sb.WriteString("  ")
		for i, s := range p.Strategies {
			if i == 0 {
				fmt.Fprintf(&sb, "[%s]", s)
			} else {
				fmt.Fprintf(&sb, " (%s)", s)
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	return sb.String()
}
