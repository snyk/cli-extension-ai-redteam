package save

import (
	"context"
	"fmt"
	"os"

	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets"
	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

var (
	WorkflowID = workflow.NewWorkflowIdentifier("redteam.targets.save")
	dataType   = workflow.NewTypeIdentifier(WorkflowID, "redteam.targets.save")
)

func RegisterWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-targets-save", pflag.ExitOnError)
	flagset.Bool(utils.FlagExperimental, false,
		"This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagTenantID, "", "Tenant ID (auto-discovered if not provided)")
	flagset.String(utils.FlagConfig, "", "Path to the red team configuration file (default: redteam.yaml)")
	flagset.String(utils.FlagTargetName, "", "Target name override")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(WorkflowID, cfg, saveWorkflow); err != nil {
		return fmt.Errorf("error while registering targets save workflow: %w", err)
	}
	return nil
}

func saveWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	return RunSaveWorkflow(invocationCtx, targets.DefaultFactory())
}

func RunSaveWorkflow(
	invocationCtx workflow.InvocationContext,
	factory targets.ControlServerFactory,
) ([]workflow.Data, error) {
	env, err := targets.InitEnv(invocationCtx, factory)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped inside InitEnv
	}

	configPath := env.Config.GetString(utils.FlagConfig)
	if configPath == "" {
		configPath = "redteam.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, redteam_errors.NewConfigValidationError(
			fmt.Sprintf("failed to read config file %s: %s", configPath, err),
		)
	}

	var rtConfig redteam.Config
	if unmarshalErr := yaml.Unmarshal(data, &rtConfig); unmarshalErr != nil {
		return nil, redteam_errors.NewConfigValidationError(
			fmt.Sprintf("failed to parse %s: %s", configPath, unmarshalErr),
		)
	}

	name := rtConfig.Target.Name
	if flagName := env.Config.GetString(utils.FlagTargetName); flagName != "" {
		name = flagName
	}
	if name == "" {
		return nil, redteam_errors.NewBadRequestError(
			"target name is required: set target.name in config or use --name",
		)
	}

	configMap, err := targets.StructToMap(&rtConfig)
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("serialize config: %s", err))
	}

	targets.StripHeaders(configMap)

	ctx := context.Background()
	resp, err := env.Client.CreateTarget(ctx, &controlserver.TargetCreateRequest{
		Name:   name,
		Config: configMap,
	})
	if err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from controlserver
	}

	msg := fmt.Sprintf("Target %q saved (id: %s)\n", resp.Name, resp.ID)
	return targets.TextOutput(dataType, msg), nil
}
