package redteam

import (
	"fmt"
	"strings"

	"github.com/snyk/go-application-framework/pkg/ui"
)

const (
	bred = "\033[91m"
	bold = "\033[1m"
	reset = "\033[0m"
)

func displayBanner(userInterface ui.UserInterface, cfg *Config) {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s%sSnyk Agent Red Teaming%s\n", bold, bred, reset))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  Target:     %s\n", cfg.Target.Settings.URL))
	sb.WriteString(fmt.Sprintf("  Server:     %s\n", cfg.ControlServerURL))
	sb.WriteString(fmt.Sprintf("  Goal:       %s\n", cfg.Goal))
	sb.WriteString(fmt.Sprintf("  Strategies: %s\n", strings.Join(cfg.Strategies, ", ")))
	sb.WriteString("\n")

	_ = userInterface.Output(sb.String()) //nolint:errcheck // best-effort display
}
