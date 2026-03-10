package redteam

import (
	"fmt"

	"github.com/snyk/go-application-framework/pkg/workflow"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteamget"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

func Init(e workflow.Engine) error {
	e.GetConfiguration().AddAlternativeKeys(utils.FlagTenantID, []string{"SNYK_TENANT_ID"})

	if err := redteam.RegisterWorkflows(e); err != nil {
		return fmt.Errorf("error registering redteam workflow: %w", err)
	}
	if err := redteamget.RegisterRedTeamGetWorkflow(e); err != nil {
		return fmt.Errorf("error registering redteam get workflow: %w", err)
	}
	return nil
}
