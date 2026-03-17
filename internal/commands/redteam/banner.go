package redteam

import (
	"fmt"
	"strings"

	"github.com/snyk/go-application-framework/pkg/ui"
)

const (
	bred  = "\033[91m"
	bold  = "\033[1m"
	reset = "\033[0m"
)

func displayBanner(userInterface ui.UserInterface, cfg *Config) {
	var sb strings.Builder

	fmt.Fprintf(&sb, "\n")
	fmt.Fprintf(&sb, "  %s%sSnyk Agent Red Teaming%s\n", bold, bred, reset)
	fmt.Fprintf(&sb, "\n")
	fmt.Fprintf(&sb, "  Target:     %s\n", cfg.Target.Settings.URL)
	if cfg.ProfileName != "" {
		fmt.Fprintf(&sb, "  Profile:    %s\n", cfg.ProfileName)
	}
	fmt.Fprintf(&sb, "  Goals:      %s\n", strings.Join(cfg.UniqueGoals(), ", "))
	fmt.Fprintf(&sb, "\n")

	_ = userInterface.Output(sb.String())
}
