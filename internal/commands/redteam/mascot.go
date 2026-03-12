package redteam

import (
	"fmt"
	"strings"

	"github.com/snyk/go-application-framework/pkg/ui"
)

const (
	red   = "\033[31m"
	bred  = "\033[91m"
	dim   = "\033[2m"
	bold  = "\033[1m"
	reset = "\033[0m"
)

func displayMascot(userInterface ui.UserInterface, cfg *Config) {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("   %s▗%s     %s▖%s\n", red, reset, red, reset))
	sb.WriteString(fmt.Sprintf("   %s▐▛███▜▌%s%s▜▘%s\n", bred, reset, dim, reset))
	sb.WriteString(fmt.Sprintf("  %s▝▜█████▛▝%s\n", bred, reset))
	sb.WriteString(fmt.Sprintf("    %s▘▘ ▝▝%s\n", bred, reset))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s%sminired%s %s- Red teaming CLI%s\n", bold, bred, reset, dim, reset))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  Target:     %s\n", cfg.Target.Settings.URL))
	sb.WriteString(fmt.Sprintf("  Goal:       %s\n", cfg.Goal))
	sb.WriteString(fmt.Sprintf("  Strategies: %s\n", strings.Join(cfg.Strategies, ", ")))
	sb.WriteString("\n")

	_ = userInterface.Output(sb.String()) //nolint:errcheck // best-effort display
}
