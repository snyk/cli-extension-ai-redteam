package delete //nolint:predeclared // sub-command package name

import (
	"context"
	"fmt"

	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets"
	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

var (
	WorkflowID = workflow.NewWorkflowIdentifier("redteam.targets.delete")
	dataType   = workflow.NewTypeIdentifier(WorkflowID, "redteam.targets.delete")
)

func RegisterWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-targets-delete", pflag.ExitOnError)
	flagset.Bool(utils.FlagExperimental, false,
		"This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagTenantID, "", "Tenant ID (auto-discovered if not provided)")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(WorkflowID, cfg, deleteWorkflow); err != nil {
		return fmt.Errorf("error while registering targets delete workflow: %w", err)
	}
	return nil
}

func deleteWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	return RunDeleteWorkflow(invocationCtx, targets.DefaultFactory())
}

func RunDeleteWorkflow(
	invocationCtx workflow.InvocationContext,
	factory targets.ControlServerFactory,
) ([]workflow.Data, error) {
	env, err := targets.InitEnv(invocationCtx, factory)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped inside InitEnv
	}

	name := targets.TargetNameFromArgs("delete")
	if name == "" {
		return nil, redteam_errors.NewBadRequestError("usage: snyk redteam targets delete <name>")
	}

	ctx := context.Background()
	if err := env.Client.DeleteTarget(ctx, name); err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from controlserver
	}

	msg := fmt.Sprintf("Target %q deleted\n", name)
	return targets.TextOutput(dataType, msg), nil
}
