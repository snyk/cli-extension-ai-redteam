package targets

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/rs/zerolog"
	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"

	"github.com/snyk/cli-extension-ai-redteam/internal/helpers"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

const (
	FlagPort   = "port"
	FlagOutput = "output"
)

// ControlServerFactory creates a controlserver.Client from the standard set of arguments.
type ControlServerFactory func(
	logger *zerolog.Logger, httpClient *http.Client, url, tenantID string,
) controlserver.Client

// DefaultFactory returns a ControlServerFactory that builds real control-server clients.
func DefaultFactory() ControlServerFactory {
	return func(logger *zerolog.Logger, httpClient *http.Client, url, tenantID string) controlserver.Client {
		return controlserver.NewClient(logger, httpClient, url, tenantID)
	}
}

// Env holds the pre-validated environment shared by every targets sub-command.
type Env struct {
	Client controlserver.Client
	Config configuration.Configuration
	InvCtx workflow.InvocationContext
}

// InitEnv performs auth / flag checks and builds the control-server client.
func InitEnv(invocationCtx workflow.InvocationContext, factory ControlServerFactory) (*Env, error) {
	config := invocationCtx.GetConfiguration()

	if err := utils.RejectOrgFlag(); err != nil {
		return nil, err //nolint:wrapcheck // already a catalog error
	}
	if err := utils.RequireAuth(config); err != nil {
		return nil, err //nolint:wrapcheck // already a catalog error
	}
	if !config.GetBool(utils.FlagExperimental) {
		return nil, cli_errors.NewCommandIsExperimentalError("re-run with --experimental to use this command")
	}

	tenantID, err := helpers.GetTenantID(invocationCtx, config.GetString(utils.FlagTenantID))
	if err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from helpers
	}

	httpClient := invocationCtx.GetNetworkAccess().GetHttpClient()
	httpClient.Timeout = controlserver.DefaultClientTimeout
	apiURL := config.GetString(configuration.API_URL)
	csClient := factory(invocationCtx.GetEnhancedLogger(), httpClient, apiURL, tenantID)

	return &Env{
		Client: csClient,
		Config: config,
		InvCtx: invocationCtx,
	}, nil
}

// TargetNameFromArgs extracts the target name positional argument from os.Args.
// It looks for the first positional arg after the given sub-command keyword.
func TargetNameFromArgs(subCommand string) string {
	args := os.Args[1:]
	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			if strings.Contains(a, "=") {
				continue
			}
			switch a {
			case "--tenant-id", "--config", "--target-name", "--port", "--output":
				i++
			}
			continue
		}
		positional = append(positional, a)
	}

	for i, p := range positional {
		if p == subCommand && i+1 < len(positional) {
			return positional[i+1]
		}
	}
	return ""
}

// StructToMap serializes any value to a map[string]any via JSON round-trip.
func StructToMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return m, nil
}

// StripHeaders removes target.settings.headers from the config map before persisting.
func StripHeaders(configMap map[string]any) {
	t, ok := configMap["target"]
	if !ok {
		return
	}
	tm, ok := t.(map[string]any)
	if !ok {
		return
	}
	s, ok := tm["settings"]
	if !ok {
		return
	}
	sm, ok := s.(map[string]any)
	if !ok {
		return
	}
	delete(sm, "headers")
}

// TextOutput wraps a text string as workflow output data.
func TextOutput(wfType workflow.Identifier, s string) []workflow.Data {
	return []workflow.Data{
		workflow.NewData(wfType, "text/plain", []byte(s)),
	}
}
