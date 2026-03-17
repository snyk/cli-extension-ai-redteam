package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/snyk/go-application-framework/pkg/devtools"

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
		fmt.Fprintln(os.Stderr, "Error: not authenticated. Run `go run ./cmd/develop auth` or set SNYK_TOKEN.")
		os.Exit(1)
	}

	cmd, err := devtools.Cmd(redteam.Init)
	if err != nil {
		log.Fatal(err) //nolint:forbidigo // CLI entry point, ui not available yet
	}
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
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
