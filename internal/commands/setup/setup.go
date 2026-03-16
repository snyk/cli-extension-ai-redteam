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
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
	"github.com/snyk/cli-extension-ai-redteam/internal/web"
)

var SetupWorkflowID = workflow.NewWorkflowIdentifier("redteam.setup")

func RegisterSetupWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-setup", pflag.ExitOnError)
	flagset.Bool(utils.FlagExperimental, false, "This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagOutput, "redteam.yaml", "Output path for the generated configuration file")
	flagset.String(utils.FlagConfig, "", "Load an existing configuration file to edit")
	flagset.Int(utils.FlagPort, 8484, "Port for the setup wizard web server")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(SetupWorkflowID, cfg, setupWorkflow); err != nil {
		return fmt.Errorf("error while registering setup workflow: %w", err)
	}
	return nil
}

func setupWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	config := invocationCtx.GetConfiguration()

	experimental := config.GetBool(utils.FlagExperimental)
	if !experimental {
		return nil, cli_errors.NewCommandIsExperimentalError("")
	}

	outputPath := config.GetString(utils.FlagOutput)
	port := config.GetInt(utils.FlagPort)
	configPath := config.GetString(utils.FlagConfig)

	var initialConfig *redteam.Config
	if configPath == "" {
		configPath = "redteam.yaml"
	}
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg redteam.Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
		}
		initialConfig = &cfg
		fmt.Printf("Loaded existing configuration from %s\n", configPath)
	} else if config.GetString(utils.FlagConfig) != "" {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	httpClient := invocationCtx.GetNetworkAccess().GetHttpClient()
	apiURL := config.GetString(configuration.API_URL)
	logger := zerolog.Nop()
	csClient := controlserver.NewClient(&logger, httpClient, apiURL, "")

	server := web.NewServer(port, outputPath, configPath, initialConfig, csClient)
	if err := server.Start(); err != nil {
		return nil, fmt.Errorf("setup wizard error: %w", err)
	}

	return nil, nil
}
