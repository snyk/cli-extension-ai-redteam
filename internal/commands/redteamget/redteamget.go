package redteamget

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"
	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam/htmlreport"
	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/helpers"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

const (
	getWorkflowName = "redteam.get"
)

var (
	GetWorkflowID   = workflow.NewWorkflowIdentifier(getWorkflowName)
	getWorkflowType = workflow.NewTypeIdentifier(GetWorkflowID, getWorkflowName)
)

type ControlServerFactory func(
	logger *zerolog.Logger, httpClient *http.Client, url, tenantID string,
) controlserver.Client

func RegisterRedTeamGetWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-get", pflag.ExitOnError)
	flagset.Bool(utils.FlagExperimental, false,
		"This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagScanID, "", "Scan ID to retrieve results for")
	flagset.Bool(utils.FlagHTML, false, "Output the red team report in HTML format instead of JSON")
	flagset.String(utils.FlagHTMLFileOutput, "", "Write the HTML report to the specified file path")
	flagset.String(utils.FlagJSONFileOutput, "", "Write the JSON report to the specified file path")
	flagset.String(utils.FlagTenantID, "", "Tenant ID (auto-discovered if not provided)")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(GetWorkflowID, cfg, redTeamGetWorkflow); err != nil {
		return fmt.Errorf("error while registering red team get workflow: %w", err)
	}
	return nil
}

func redTeamGetWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	factory := ControlServerFactory(
		func(logger *zerolog.Logger, httpClient *http.Client, url, tenantID string) controlserver.Client {
			return controlserver.NewClient(logger, httpClient, url, tenantID)
		})
	return RunRedTeamGetWorkflow(invocationCtx, factory)
}

func RunRedTeamGetWorkflow(
	invocationCtx workflow.InvocationContext,
	controlServerFactory ControlServerFactory,
) ([]workflow.Data, error) {
	logger := invocationCtx.GetEnhancedLogger()
	config := invocationCtx.GetConfiguration()

	if err := utils.RequireAuth(config); err != nil {
		logger.Debug().Msg("No organization id is found.")
		return nil, err //nolint:wrapcheck // already a catalog error
	}

	experimental := config.GetBool(utils.FlagExperimental)
	if !experimental {
		logger.Debug().Msg("Required experimental flag is not present")
		return nil, cli_errors.NewCommandIsExperimentalError("re-run with --experimental to use this command")
	}

	snykAPIURL := config.GetString(configuration.API_URL)

	tenantID, err := helpers.GetTenantID(invocationCtx, config.GetString(utils.FlagTenantID))
	if err != nil {
		return nil, err //nolint:wrapcheck // returned by helpers.GetTenantID
	}

	return handleGetScanResults(invocationCtx, controlServerFactory, tenantID, snykAPIURL)
}

func handleGetScanResults(
	invocationCtx workflow.InvocationContext,
	controlServerFactory ControlServerFactory,
	tenantID string,
	snykAPIURL string,
) ([]workflow.Data, error) {
	logger := invocationCtx.GetEnhancedLogger()
	config := invocationCtx.GetConfiguration()
	httpClient := invocationCtx.GetNetworkAccess().GetHttpClient()
	ctx := context.Background()

	scanID := config.GetString(utils.FlagScanID)
	if scanID == "" {
		return nil, redteam_errors.NewBadRequestError("No scan ID specified")
	}

	validate := validator.New()
	if err := validate.Var(scanID, "uuid"); err != nil {
		return nil, redteam_errors.NewBadRequestError(fmt.Sprintf("Scan ID is not a valid UUID: %q", scanID))
	}

	snykAPIClient := controlServerFactory(logger, httpClient, snykAPIURL, tenantID)

	logger.Debug().Str("scanID", scanID).Msg("Fetching scan report")

	reportJSON, reportErr := snykAPIClient.GetReport(ctx, scanID)
	if reportErr != nil {
		logger.Debug().Err(reportErr).Msg("Error fetching scan report")
		return nil, redteam_errors.NewGenericRedTeamError(reportErr.Error(), reportErr)
	}

	jsonResults := []workflow.Data{workflow.NewData(getWorkflowType, "application/json", []byte(reportJSON))}

	output, htmlErr := htmlreport.ProcessResults(logger, config, jsonResults)
	if htmlErr != nil {
		return nil, redteam_errors.NewGenericRedTeamError("HTML report error", htmlErr)
	}
	return output, nil
}
