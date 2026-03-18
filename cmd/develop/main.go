package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/snyk/error-catalog-golang-public/snyk_errors"
	"github.com/snyk/go-application-framework/pkg/auth"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/devtools"
	"github.com/snyk/go-application-framework/pkg/workflow"

	"github.com/snyk/cli-extension-ai-redteam/pkg/redteam"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	for _, arg := range os.Args[1:] {
		if arg == "--debug" || arg == "-d" {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			break
		}
	}

	if !isAuthCommand() && !isAuthenticated() {
		if os.Getenv("SNYK_API") != "" {
			fmt.Fprintln(os.Stderr,
				"Error: not authenticated. Set SNYK_TOKEN "+
					"(SNYK_API is set, so stored OAuth "+
					"credentials are ignored).")
		} else {
			fmt.Fprintln(os.Stderr,
				"Error: not authenticated. Run "+
					"`go run ./cmd/develop auth` "+
					"or set SNYK_TOKEN.")
		}
		os.Exit(1)
	}

	cmd, err := devtools.Cmd(func(e workflow.Engine) error {
		// A stored OAuth token in the shared configstore has an
		// audience claim that the framework uses as the API URL,
		// ignoring SNYK_API. Clear it so SNYK_TOKEN is used instead.
		if apiURL := os.Getenv("SNYK_API"); apiURL != "" {
			e.GetConfiguration().Set(auth.CONFIG_KEY_OAUTH_TOKEN, "")
			e.GetConfiguration().Set(configuration.API_URL, apiURL)
		}
		return redteam.Init(e)
	})
	if err != nil {
		log.Fatal(err) //nolint:forbidigo // dev harness, no ui available yet
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, formatError(err))
		os.Exit(1)
	}
}

func isAuthCommand() bool {
	for _, arg := range os.Args[1:] {
		if arg == "auth" {
			return true
		}
	}
	return false
}

// isAuthenticated checks for a Snyk auth token in environment variables
// or in the config store (~/.config/configstore/snyk.json).
// This mimics the regular Snyk CLI which requires authentication before
// running any command, but the devtools harness doesn't enforce it.
func isAuthenticated() bool {
	if os.Getenv("SNYK_TOKEN") != "" || os.Getenv("SNYK_OAUTH_TOKEN") != "" {
		return true
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	data, err := os.ReadFile(filepath.Join(home, ".config", "configstore", "snyk.json"))
	if err != nil {
		return false
	}

	var cfg map[string]string
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false
	}

	return cfg["api"] != "" || cfg["INTERNAL_OAUTH_TOKEN_STORAGE"] != ""
}

// formatError mimics the Snyk CLI's error display: if the error chain
// contains a snyk_errors.Error, show its Detail (the user-facing message)
// rather than just the catalog Title.
func formatError(err error) string {
	var snykErr snyk_errors.Error
	if errors.As(err, &snykErr) {
		if snykErr.Detail != "" {
			return fmt.Sprintf("Error: %s", snykErr.Detail)
		}
		return fmt.Sprintf("Error: %s", snykErr.Title)
	}
	return fmt.Sprintf("Error: %s", err)
}
