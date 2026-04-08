package get

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets"
	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
	"github.com/snyk/cli-extension-ai-redteam/internal/wizard"
)

var (
	WorkflowID = workflow.NewWorkflowIdentifier("redteam.targets.get")
	dataType   = workflow.NewTypeIdentifier(WorkflowID, "redteam.targets.get")
)

func RegisterWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-targets-get", pflag.ExitOnError)
	flagset.Bool(utils.FlagExperimental, false,
		"This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagTenantID, "", "Tenant ID (auto-discovered if not provided)")
	flagset.Int(targets.FlagPort, 8484, "Port for the setup wizard web server")
	flagset.String(targets.FlagOutput, "", "Write target config to a file instead of opening the wizard")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(WorkflowID, cfg, getWorkflow); err != nil {
		return fmt.Errorf("error while registering targets get workflow: %w", err)
	}
	return nil
}

func getWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	return RunGetWorkflow(invocationCtx, targets.DefaultFactory())
}

func RunGetWorkflow(
	invocationCtx workflow.InvocationContext,
	factory targets.ControlServerFactory,
) ([]workflow.Data, error) {
	env, err := targets.InitEnv(invocationCtx, factory)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped inside InitEnv
	}

	name := targets.TargetNameFromArgs("get")
	if name == "" {
		return nil, redteam_errors.NewBadRequestError("usage: snyk redteam targets get <name>")
	}

	ctx := context.Background()
	resp, err := env.Client.GetTarget(ctx, name)
	if err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from controlserver
	}

	rtConfig, err := configFromMap(resp.Config)
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("failed to parse target config: %s", err))
	}

	if rtConfig.Target.Name == "" {
		rtConfig.Target.Name = resp.Name
	}

	yamlBytes, err := yaml.Marshal(rtConfig)
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("marshal config to YAML: %s", err))
	}

	if outputPath := env.Config.GetString(targets.FlagOutput); outputPath != "" {
		if writeErr := os.WriteFile(outputPath, yamlBytes, 0o600); writeErr != nil {
			return nil, redteam_errors.NewInternalError(fmt.Sprintf("write config to %s: %s", outputPath, writeErr))
		}
		msg := fmt.Sprintf("Target %q config written to %s\n", resp.Name, outputPath)
		return targets.TextOutput(dataType, msg), nil
	}

	tmpDir, err := os.MkdirTemp("", "redteam-target-*")
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("create temp dir: %s", err))
	}

	configPath := tmpDir + "/redteam.yaml"
	if err := os.WriteFile(configPath, yamlBytes, 0o600); err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("write temp config: %s", err))
	}

	port := env.Config.GetInt(targets.FlagPort)
	apiURL := env.Config.GetString(configuration.API_URL)
	logger := zerolog.Nop()
	httpClient := env.InvCtx.GetNetworkAccess().GetHttpClient()
	httpClient.Timeout = controlserver.DefaultClientTimeout
	csClient := controlserver.NewClient(&logger, httpClient, apiURL, "")

	userInterface := env.InvCtx.GetUserInterface()
	server := wizard.NewServer(port, configPath, rtConfig, csClient, userInterface)
	if err := server.Start(); err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("setup wizard error: %s", err))
	}

	return nil, nil
}

func configFromMap(m map[string]any) (*redteam.Config, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal config map: %w", err)
	}
	var cfg redteam.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
