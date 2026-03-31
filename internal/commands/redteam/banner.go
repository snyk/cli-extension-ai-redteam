package redteam

import (
	"errors"
	"fmt"
	"strings"

	"github.com/snyk/go-application-framework/pkg/ui"
)

// scanBannerOptions configures the post-scan-creation header (mockup-style).
type scanBannerOptions struct {
	ScanID      string
	ProfileName string
	ConfigPath  string
	ScanMode    string
	TenantID    string
}

func displayScanBanner(userInterface ui.UserInterface, theme *cliTheme, cfg *Config, opts *scanBannerOptions) error {
	if opts == nil {
		return fmt.Errorf("scan banner: %w", errNilBannerOptions)
	}
	tw := terminalWidth()
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(renderEVOLogo(theme))
	sb.WriteString("\n\n")
	sb.WriteString("  ")
	sb.WriteString(theme.title().Render("AI Red Teaming"))
	sb.WriteString("\n  ")
	sb.WriteString(theme.subtitle().Render("Adversarial testing for AI-native applications"))
	sb.WriteString("\n  ")
	sb.WriteString(theme.subtitle().Render("scan id "))
	sb.WriteString(theme.muted().Render(opts.ScanID))
	sb.WriteString("\n\n")

	sb.WriteString(horizontalRule(theme, "scan configuration", tw))
	sb.WriteString("\n")
	sb.WriteString(kvLine(theme, "Target", theme.accent().Render(cfg.Target.Settings.URL)))
	goals := strings.Join(cfg.UniqueGoals(), ", ")
	if goals != "" {
		sb.WriteString(kvLine(theme, "Goals", theme.subtitle().Render(goals)))
	}
	sb.WriteString(kvLine(theme, "Config", theme.subtitle().Render(opts.ConfigPath)))
	if opts.ProfileName != "" {
		sb.WriteString(kvLine(theme, "Profile", theme.subtitle().Render(opts.ProfileName)))
	}
	if err := userInterface.Output(sb.String()); err != nil {
		return fmt.Errorf("scan banner output: %w", err)
	}
	return nil
}

var errNilBannerOptions = errors.New("nil banner options")

func kvLine(theme *cliTheme, key, valueStyled string) string {
	return fmt.Sprintf("  %s %s\n", theme.label().Render(key+":"), valueStyled)
}
