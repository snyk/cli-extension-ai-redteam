package web

import (
	"net/http"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
)

// HandleGetInitialConfig exports handleGetInitialConfig for external tests.
func HandleGetInitialConfig(cfg *redteam.Config, configPath string) http.HandlerFunc {
	return handleGetInitialConfig(cfg, configPath)
}

// HandlePing exports handlePing for external tests.
func HandlePing() http.HandlerFunc {
	return handlePing()
}

// HandleListGoals exports handleListGoals for external tests.
func HandleListGoals(client controlserver.Client) http.HandlerFunc {
	return handleListGoals(client)
}

// HandleListStrategies exports handleListStrategies for external tests.
func HandleListStrategies(client controlserver.Client) http.HandlerFunc {
	return handleListStrategies(client)
}

// InitialConfigResponse is an alias for external tests.
type InitialConfigResponse = initialConfigResponse

// PingRequest is an alias for external tests.
type PingRequest = pingRequest
