package redteam

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/snyk/go-application-framework/pkg/ui"
)

var (
	bannerTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	bannerLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#5d6d7e"))
	bannerValue = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
)

func displayBanner(userInterface ui.UserInterface, cfg *Config, profileName string) {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s\n", bannerTitle.Render("SNYK AGENT RED TEAMING")))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s %s\n", bannerLabel.Render("Target:    "), bannerValue.Render(cfg.Target.Settings.URL)))
	if profileName != "" {
		sb.WriteString(fmt.Sprintf("  %s %s\n", bannerLabel.Render("Profile:   "), bannerValue.Render(profileName)))
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n", bannerLabel.Render("Goals:     "), bannerValue.Render(strings.Join(cfg.UniqueGoals(), ", "))))
	sb.WriteString("\n")

	_ = userInterface.Output(sb.String()) //nolint:errcheck // best-effort banner output
}
