package list

import (
	"context"
	"fmt"
	"strings"

	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

var (
	WorkflowID = workflow.NewWorkflowIdentifier("redteam.targets.list")
	dataType   = workflow.NewTypeIdentifier(WorkflowID, "redteam.targets.list")
)

func RegisterWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-targets-list", pflag.ExitOnError)
	flagset.Bool(utils.FlagExperimental, false,
		"This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagTenantID, "", "Tenant ID (auto-discovered if not provided)")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(WorkflowID, cfg, listWorkflow); err != nil {
		return fmt.Errorf("error while registering targets list workflow: %w", err)
	}
	return nil
}

func listWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	return RunListWorkflow(invocationCtx, targets.DefaultFactory())
}

func RunListWorkflow(
	invocationCtx workflow.InvocationContext,
	factory targets.ControlServerFactory,
) ([]workflow.Data, error) {
	env, err := targets.InitEnv(invocationCtx, factory)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped inside InitEnv
	}

	ctx := context.Background()
	items, err := env.Client.ListTargets(ctx)
	if err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from controlserver
	}

	if len(items) == 0 {
		return targets.TextOutput(dataType, "No saved targets found.\n"), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%-30s %-38s %s\n", "NAME", "ID", "UPDATED")
	for _, item := range items {
		fmt.Fprintf(&sb, "%-30s %-38s %s\n", item.Name, item.ID, item.UpdatedAt)
	}
	return targets.TextOutput(dataType, sb.String()), nil
}
