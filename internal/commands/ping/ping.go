package ping

import (
	"context"
	"fmt"

	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

var PingWorkflowID = workflow.NewWorkflowIdentifier("redteam.ping")

func RegisterPingWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-ping", pflag.ExitOnError)
	utils.AddTargetFlags(flagset)

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(PingWorkflowID, cfg, pingWorkflow); err != nil {
		return fmt.Errorf("error while registering ping workflow: %w", err)
	}
	return nil
}

// pingWorkflow sends a test request to the configured target endpoint to verify
// connectivity and correct request/response configuration before running a full red team scan.
func pingWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
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

	logger := invocationCtx.GetEnhancedLogger()
	rtConfig, earlyReturn, err := redteam.LoadAndValidateConfig(logger, config)
	if err != nil {
		return nil, err //nolint:wrapcheck // returned by helpers.GetTenantID
	}
	if earlyReturn != nil {
		return earlyReturn, nil
	}

	headers := rtConfig.HeadersMap()
	client := target.NewHTTPClient(
		nil,
		rtConfig.Target.Settings.URL,
		headers,
		rtConfig.Target.Settings.RequestBodyTemplate,
		rtConfig.Target.Settings.ResponseSelector,
	)

	ctx := context.Background()
	result := client.Ping(ctx)

	var output string
	if result.Success {
		output = fmt.Sprintf("SUCCESS: %s\nResponse: %s", result.Suggestion, result.Response)
	} else {
		output = fmt.Sprintf("FAILED: %s\nError: %s", result.Suggestion, result.Error)
		if result.RawBody != "" {
			output += fmt.Sprintf("\nRaw body: %s", result.RawBody)
		}
	}

	return []workflow.Data{
		workflow.NewData(
			workflow.NewTypeIdentifier(PingWorkflowID, "ping"),
			"text/plain",
			[]byte(output),
		),
	}, nil
}
