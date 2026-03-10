package redteamget

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam/htmlreport"
	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/helpers"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/normalizer"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

const (
	getWorkflowName     = "redteam.get"
	envControlServerURL = "CONTROL_SERVER_URL"
	defaultCSURL        = "http://localhost:8085"
)

var (
	GetWorkflowID   = workflow.NewWorkflowIdentifier(getWorkflowName)
	getWorkflowType = workflow.NewTypeIdentifier(GetWorkflowID, getWorkflowName)
)

type CSFactory func(url, tenantID string) controlserver.Client

func RegisterRedTeamGetWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-get", pflag.ExitOnError)
	flagset.Bool(utils.FlagExperimental, false, "This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagScanID, "", "Scan ID to retrieve results for")
	flagset.Bool(utils.FlagHTML, false, "Output the red team report in HTML format instead of JSON")
	flagset.String(utils.FlagHTMLFileOutput, "", "Write the HTML report to the specified file path")
	flagset.String(utils.FlagControlServer, defaultCSURL, "URL of the minired control server")
	flagset.String(utils.FlagTenantID, "", "Tenant ID (auto-discovered if not provided)")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(GetWorkflowID, cfg, redTeamGetWorkflow); err != nil {
		return fmt.Errorf("error while registering red team get workflow: %w", err)
	}
	return nil
}

func redTeamGetWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	logger := invocationCtx.GetEnhancedLogger()
	httpClient := invocationCtx.GetNetworkAccess().GetHttpClient()
	factory := func(url, tenantID string) controlserver.Client {
		return controlserver.NewClient(logger, httpClient, url, tenantID)
	}
	return RunRedTeamGetWorkflow(invocationCtx, factory)
}

func RunRedTeamGetWorkflow(
	invocationCtx workflow.InvocationContext,
	csFactory CSFactory,
) ([]workflow.Data, error) {
	logger := invocationCtx.GetEnhancedLogger()
	config := invocationCtx.GetConfiguration()

	experimental := config.GetBool(utils.FlagExperimental)
	if !experimental {
		logger.Debug().Msg("Required experimental flag is not present")
		return nil, cli_errors.NewCommandIsExperimentalError("")
	}

	tenantID, err := helpers.GetTenantID(invocationCtx, config.GetString(utils.FlagTenantID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tenant: %w", err)
	}

	return handleGetScanResults(invocationCtx, csFactory, tenantID)
}

func handleGetScanResults(
	invocationCtx workflow.InvocationContext,
	csFactory CSFactory,
	tenantID string,
) ([]workflow.Data, error) {
	logger := invocationCtx.GetEnhancedLogger()
	config := invocationCtx.GetConfiguration()
	ctx := context.Background()

	scanID := config.GetString(utils.FlagScanID)
	if scanID == "" {
		return nil, redteam_errors.NewBadRequestError("No scan ID specified")
	}

	validate := validator.New()
	if err := validate.Var(scanID, "uuid"); err != nil {
		return nil, redteam_errors.NewBadRequestError(fmt.Sprintf("Scan ID is not a valid UUID: %q", scanID))
	}

	csURL := config.GetString(utils.FlagControlServer)
	if csURL == "" {
		if v := os.Getenv(envControlServerURL); v != "" {
			csURL = v
		} else {
			csURL = defaultCSURL
		}
	}
	csClient := csFactory(csURL, tenantID)

	logger.Debug().Str("scanID", scanID).Msg("Fetching scan results")

	status, statusErr := csClient.GetStatus(ctx, scanID)
	if statusErr != nil {
		logger.Debug().Err(statusErr).Msg("failed to get status, continuing without summary")
	}

	result, resultErr := csClient.GetResult(ctx, scanID)
	if resultErr != nil {
		logger.Debug().Err(resultErr).Msg("Error fetching scan result")
		return nil, redteam_errors.NewGenericRedTeamError(resultErr.Error(), resultErr)
	}

	normalized := normalizer.Normalize(result, status, "")

	resultsBytes, err := json.Marshal(normalized)
	if err != nil {
		logger.Debug().Err(err).Msg("Error marshaling scan results")
		return nil, redteam_errors.NewGenericRedTeamError("Failed processing scan results", err)
	}

	jsonResults := []workflow.Data{workflow.NewData(getWorkflowType, "application/json", resultsBytes)}

	output, htmlErr := htmlreport.ProcessResults(logger, config, jsonResults)
	if htmlErr != nil {
		return nil, redteam_errors.NewGenericRedTeamError("HTML report error", htmlErr)
	}
	return output, nil
}
