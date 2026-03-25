package setup

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
	"github.com/snyk/cli-extension-ai-redteam/internal/wizard"
)

var SetupWorkflowID = workflow.NewWorkflowIdentifier("redteam.setup")

func RegisterSetupWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-setup", pflag.ExitOnError)
	const flagPort = "port"

	flagset.Bool(utils.FlagExperimental, false,
		"This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagConfig, "", "Load an existing configuration file to edit")
	flagset.Int(flagPort, 8484, "Port for the setup wizard web server")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(SetupWorkflowID, cfg, setupWorkflow); err != nil {
		return fmt.Errorf("error while registering setup workflow: %w", err)
	}
	return nil
}

// setupWorkflow launches a local web server that serves the setup wizard UI.
// The wizard walks the user through configuring a red team target (endpoint, headers,
// goals, strategies) and produces a YAML configuration file for `snyk redteam`.
func setupWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	config := invocationCtx.GetConfiguration()

	if err := utils.RejectOrgFlag(); err != nil {
		return nil, err //nolint:wrapcheck // already a catalog error
	}

	if err := utils.RequireAuth(config); err != nil {
		return nil, err //nolint:wrapcheck // already a catalog error
	}

	experimental := config.GetBool(utils.FlagExperimental)
	if !experimental {
		return nil, cli_errors.NewCommandIsExperimentalError("re-run with --experimental to use this command")
	}

	port := config.GetInt("port")
	configPath := config.GetString(utils.FlagConfig)
	logger := zerolog.Nop()

	var initialConfig *redteam.Config
	if configPath == "" {
		configPath = "redteam.yaml"
	}
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg redteam.Config
		if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
			return nil, redteam_errors.NewConfigValidationError(fmt.Sprintf("failed to parse %s: %s", configPath, unmarshalErr))
		}
		initialConfig = &cfg
		logger.Info().Str("path", configPath).Msg("loaded existing configuration")
	} else if config.GetString(utils.FlagConfig) != "" {
		return nil, redteam_errors.NewConfigValidationError(fmt.Sprintf("failed to read config file %s: %s", configPath, err))
	}

	httpClient := invocationCtx.GetNetworkAccess().GetHttpClient()
	httpClient.Timeout = controlserver.DefaultClientTimeout
	apiURL := config.GetString(configuration.API_URL)
	csClient := controlserver.NewClient(&logger, httpClient, apiURL, "")

	userInterface := invocationCtx.GetUserInterface()
	server := wizard.NewServer(port, configPath, initialConfig, csClient, userInterface)
	if err := server.Start(); err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("setup wizard error: %s", err))
	}

	return nil, nil
}
