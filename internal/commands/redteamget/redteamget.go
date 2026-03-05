package redteamget

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	snyk_common_errors "github.com/snyk/error-catalog-golang-public/snyk"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam/htmlreport"
	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/normalizer"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

const (
	getWorkflowName = "redteam.get"
	defaultCSURL    = "http://localhost:8085"
)

var (
	GetWorkflowID   = workflow.NewWorkflowIdentifier(getWorkflowName)
	getWorkflowType = workflow.NewTypeIdentifier(GetWorkflowID, getWorkflowName)
)

type CSFactory func(url string) controlserver.Client

func RegisterRedTeamGetWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-get", pflag.ExitOnError)
	flagset.Bool(utils.FlagExperimental, false, "This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagScanID, "", "Scan ID to retrieve results for")
	flagset.Bool(utils.FlagHTML, false, "Output the red team report in HTML format instead of JSON")
	flagset.String(utils.FlagHTMLFileOutput, "", "Write the HTML report to the specified file path")
	flagset.String(utils.FlagControlServer, defaultCSURL, "URL of the minired control server")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(GetWorkflowID, cfg, redTeamGetWorkflow); err != nil {
		return fmt.Errorf("error while registering red team get workflow: %w", err)
	}
	return nil
}

func redTeamGetWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	logger := invocationCtx.GetEnhancedLogger()
	factory := func(url string) controlserver.Client {
		return controlserver.NewClient(logger, &http.Client{}, url)
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

	orgID := config.GetString(configuration.ORGANIZATION)
	if orgID == "" {
		logger.Debug().Msg("No organization id is found.")
		return nil, snyk_common_errors.NewUnauthorisedError("")
	}

	return handleGetScanResults(invocationCtx, csFactory)
}

func handleGetScanResults(
	invocationCtx workflow.InvocationContext,
	csFactory CSFactory,
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
		csURL = defaultCSURL
	}
	csClient := csFactory(csURL)

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
