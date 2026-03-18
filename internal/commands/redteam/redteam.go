package redteam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog"
	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/ui"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam/clireport"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam/htmlreport"
	"github.com/snyk/cli-extension-ai-redteam/internal/helpers"
	"github.com/snyk/cli-extension-ai-redteam/internal/models"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/normalizer"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

var WorkflowID = workflow.NewWorkflowIdentifier("redteam")

type (
	ControlServerFactory func(logger *zerolog.Logger, httpClient *http.Client, url, tenantID string) controlserver.Client
	TargetFactory        func(httpClient *http.Client, url string, headers map[string]string, bodyTemplate, responseSelector string) target.Client
)

var DefaultSnykAPIFactory ControlServerFactory = func(logger *zerolog.Logger, httpClient *http.Client, url, tenantID string) controlserver.Client {
	return controlserver.NewClient(logger, httpClient, url, tenantID)
}

var DefaultTargetFactory TargetFactory = func(
	httpClient *http.Client, url string, headers map[string]string, bodyTemplate, responseSelector string,
) target.Client {
	return target.NewHTTPClient(httpClient, url, headers, bodyTemplate, responseSelector)
}

func RegisterWorkflows(e workflow.Engine) error {
	if err := RegisterRedTeamWorkflow(e); err != nil {
		return fmt.Errorf("error while registering red team workflow: %w", err)
	}
	return nil
}

func RegisterRedTeamWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam", pflag.ExitOnError)
	utils.AddTargetFlags(flagset)
	flagset.Bool(utils.FlagHTML, false, "Output the red team report in HTML format instead of JSON")
	flagset.String(utils.FlagHTMLFileOutput, "", "Write the HTML report to the specified file path")
	flagset.Bool(utils.FlagFullConversation, false, "Show all conversation turns in findings (default: first and last only)")
	flagset.Bool(utils.FlagJSON, false, "Output raw JSON instead of the styled CLI report")
	flagset.Bool(utils.FlagListGoals, false, "List all available attack goals and exit")
	flagset.Bool(utils.FlagListStrategies, false, "List all available attack strategies and exit")
	flagset.String(utils.FlagTenantID, "", "Tenant ID (auto-discovered if not provided)")
	flagset.String(utils.FlagPurpose, "", "Intended purpose of the target (ground truth for the judge)")
	flagset.String(utils.FlagSystemPrompt, "", "Target system prompt (ground truth for prompt-extraction scoring)")
	flagset.StringArray(utils.FlagTools, nil, "Tool names the target is configured with (ground truth, repeatable)")
	flagset.Bool(utils.FlagReport, false, "Re-open the last scan report without running a new scan")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(WorkflowID, cfg, redTeamWorkflow); err != nil {
		return fmt.Errorf("error while registering red team workflow: %w", err)
	}
	return nil
}

func redTeamWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	return RunRedTeamWorkflow(invocationCtx, DefaultSnykAPIFactory, DefaultTargetFactory)
}

func RunRedTeamWorkflow(
	invocationCtx workflow.InvocationContext,
	controlServerFactory ControlServerFactory,
	targetFactory TargetFactory,
) ([]workflow.Data, error) {
	logger := invocationCtx.GetEnhancedLogger()
	config := invocationCtx.GetConfiguration()

	config.Set(configuration.RAW_CMD_ARGS, os.Args[1:])

	experimental := config.GetBool(utils.FlagExperimental)
	if !experimental {
		logger.Debug().Msg("Required experimental flag is not present")
		return nil, cli_errors.NewCommandIsExperimentalError("")
	}

	if config.GetBool(utils.FlagReport) {
		data, meta, err := clireport.LoadReport()
		if err != nil {
			return nil, fmt.Errorf("failed to load report: %w", err)
		}
		if err := clireport.RunInteractive(data, meta); err != nil {
			report := clireport.Render(data, meta)
			return []workflow.Data{newWorkflowData(contentTypePlain, []byte(report))}, nil
		}
		return []workflow.Data{newWorkflowData(contentTypePlain, []byte(""))}, nil
	}

	listGoals := config.GetBool(utils.FlagListGoals)
	listStrategies := config.GetBool(utils.FlagListStrategies)
	if listGoals || listStrategies {
		httpClient := invocationCtx.GetNetworkAccess().GetHttpClient()
		return handleListFlags(config, controlServerFactory, logger, httpClient, listGoals, listStrategies)
	}

	rtConfig, configData, err := LoadAndValidateConfig(logger, config)
	if configData != nil {
		return configData, nil
	}
	if err != nil {
		return nil, err
	}

	tenantID, err := helpers.GetTenantID(invocationCtx, config.GetString(utils.FlagTenantID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tenant: %w", err)
	}

	userInterface := invocationCtx.GetUserInterface()
	displayBanner(userInterface, rtConfig)

	targetHTTPClient := &http.Client{Timeout: target.DefaultTimeout}
	controlServerHTTPClient := invocationCtx.GetNetworkAccess().GetHttpClient()
	controlServerHTTPClient.Timeout = 15 * time.Second
	controlServerURL := config.GetString(configuration.API_URL)

	controlServerClient := controlServerFactory(logger, controlServerHTTPClient, controlServerURL, tenantID)
	targetClient := targetFactory(
		targetHTTPClient,
		rtConfig.Target.Settings.URL,
		rtConfig.HeadersMap(),
		rtConfig.Target.Settings.RequestBodyTemplate,
		rtConfig.Target.Settings.ResponseSelector,
	)

	results, normalized, scanErr := runClientDrivenScan(invocationCtx, controlServerClient, targetClient, rtConfig)
	if scanErr != nil {
		return nil, scanErr
	}

	output, htmlErr := htmlreport.ProcessResults(logger, config, results)
	if htmlErr != nil {
		return nil, fmt.Errorf("HTML report error: %w", htmlErr)
	}

	returnJSON := config.GetBool(utils.FlagJSON)
	returnHTML := config.GetBool(utils.FlagHTML)
	if !returnJSON && !returnHTML && normalized != nil {
		meta := clireport.ScanMeta{
			TargetURL:  rtConfig.Target.Settings.URL,
			Goals:      rtConfig.Goals,
			Strategies: rtConfig.Strategies,
		}
		// Save report for later re-display via --report.
		if saveErr := clireport.SaveReport(normalized, meta); saveErr != nil {
			logger.Debug().Err(saveErr).Msg("failed to save report for --report flag")
		}
		if err := clireport.RunInteractive(normalized, meta); err != nil {
			// Fallback to static report if TUI fails (e.g. piped output).
			report := clireport.Render(normalized, meta)
			return []workflow.Data{newWorkflowData(contentTypePlain, []byte(report))}, nil
		}
		return []workflow.Data{newWorkflowData(contentTypePlain, []byte(""))}, nil
	}

	return output, nil
}

func handleListFlags(
	config configuration.Configuration,
	controlServerFactory ControlServerFactory,
	logger *zerolog.Logger,
	httpClient *http.Client,
	listGoals, listStrategies bool,
) ([]workflow.Data, error) {
	ctx := context.Background()
	snykAPIURL := config.GetString(configuration.API_URL)
	controlServerClient := controlServerFactory(logger, httpClient, snykAPIURL, "")

	var lines []string
	if listGoals {
		goals, err := controlServerClient.ListGoals(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list goals: %w", err)
		}
		sort.Slice(goals, func(i, j int) bool { return goals[i].DisplayOrder < goals[j].DisplayOrder })
		lines = append(lines, "Available goals:", "")
		lines = appendEnumTable(lines, goals)
	}
	if listStrategies {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		strategies, err := controlServerClient.ListStrategies(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list strategies: %w", err)
		}
		sort.Slice(strategies, func(i, j int) bool { return strategies[i].DisplayOrder < strategies[j].DisplayOrder })
		lines = append(lines, "Available strategies:", "")
		lines = appendEnumTable(lines, strategies)
	}

	output := strings.Join(lines, "\n") + "\n"
	return []workflow.Data{newWorkflowData(contentTypePlain, []byte(output))}, nil
}

func appendEnumTable(lines []string, entries []controlserver.EnumEntry) []string {
	nameWidth := len("NAME")
	for _, e := range entries {
		if len(e.Value) > nameWidth {
			nameWidth = len(e.Value)
		}
	}
	nameWidth += 2 // padding

	lines = append(lines, fmt.Sprintf("  %-*s  %s", nameWidth, "NAME", "DESCRIPTION"))
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("  %-*s  %s", nameWidth, e.Value, e.Description))
	}
	return lines
}

func runClientDrivenScan(
	invocationCtx workflow.InvocationContext,
	csClient controlserver.Client,
	targetClient target.Client,
	rtConfig *Config,
) ([]workflow.Data, *models.GetAIVulnerabilitiesResponseData, error) {
	logger := invocationCtx.GetEnhancedLogger()
	userInterface := invocationCtx.GetUserInterface()
	ctx := context.Background()

	scanID, err := csClient.CreateScan(ctx, rtConfig.ToCreateScanRequest())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create scan: %w", err)
	}
	logger.Info().Str("scanID", scanID).Msg("scan created on Snyk API")

	progressBar := userInterface.NewProgressBar()
	progressBar.SetTitle(fmt.Sprintf("Scanning %s...", rtConfig.Target.Name))
	_ = progressBar.UpdateProgress(ui.InfiniteProgress) //nolint:errcheck // best-effort progress
	defer func() { _ = progressBar.Clear() }()          //nolint:errcheck // best-effort cleanup

	var responses []controlserver.ChatResponse
	for {
		chats, nextErr := csClient.NextChats(ctx, scanID, responses)
		if nextErr != nil {
			return nil, nil, fmt.Errorf("failed to get next chats: %w", nextErr)
		}
		if len(chats) == 0 {
			break
		}

		responses = make([]controlserver.ChatResponse, 0, len(chats))
		for _, chat := range chats {
			resp, tgtErr := targetClient.SendPrompt(ctx, chat.Prompt)
			if tgtErr != nil {
				if errors.Is(tgtErr, target.ErrCircuitOpen) {
					return nil, nil, fmt.Errorf("aborting scan: %w", tgtErr)
				}
				logger.Warn().Err(tgtErr).Str("chatID", chat.ChatID).Msg("target error, using error as response")
				resp = fmt.Sprintf("[error: %s]", tgtErr.Error())
			}
			responses = append(responses, controlserver.ChatResponse{
				Seq:      chat.Seq,
				Response: resp,
			})
		}

		updateProgress(ctx, csClient, scanID, progressBar, userInterface, logger)
	}

	progressBar.SetTitle("Scan completed")
	_ = progressBar.UpdateProgress(1.0) //nolint:errcheck // best-effort progress

	status, statusErr := csClient.GetStatus(ctx, scanID)
	if statusErr != nil {
		logger.Debug().Err(statusErr).Msg("failed to get final status")
	}

	result, resultErr := csClient.GetResult(ctx, scanID)
	if resultErr != nil {
		return nil, nil, fmt.Errorf("failed to get scan result: %w", resultErr)
	}

	outputStatus(userInterface, logger, status)

	normalized := normalizer.Normalize(result, status, rtConfig.Target.Settings.URL)

	resultsBytes, marshalErr := json.Marshal(normalized)
	if marshalErr != nil {
		return nil, nil, fmt.Errorf("failed to marshal results: %w", marshalErr)
	}

	return []workflow.Data{newWorkflowData("application/json", resultsBytes)}, normalized, nil
}

func updateProgress(
	ctx context.Context,
	csClient controlserver.Client,
	scanID string,
	progressBar ui.ProgressBar,
	_ ui.UserInterface,
	logger *zerolog.Logger,
) {
	status, err := csClient.GetStatus(ctx, scanID)
	if err != nil {
		logger.Debug().Err(err).Msg("failed to get status during scan")
		return
	}
	if status.TotalChats > 0 {
		progressBar.SetTitle(fmt.Sprintf("Scanning (%d/%d)", status.Completed, status.TotalChats))
		_ = progressBar.UpdateProgress(float64(status.Completed) / float64(status.TotalChats)) //nolint:errcheck // best-effort
	}
}

func outputStatus(userInterface ui.UserInterface, logger *zerolog.Logger, status *controlserver.ScanStatus) {
	if status == nil {
		return
	}
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#7BF1A8"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#E44A50"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f8c8d"))
	msg := fmt.Sprintf("\nScan complete: %d/%d probes | %s | %s",
		status.Completed, status.TotalChats,
		red.Render(fmt.Sprintf("%d finding candidates", status.Successful)),
		green.Render(fmt.Sprintf("%d blocked", status.Failed)))
	evoLink := "\033]8;;https://evo.snyk.io/report\033\\https://evo.snyk.io/report\033]8;;\033\\"
	msg += "\n" + dim.Render("Tip: Re-open this report anytime with --report or view at ") + evoLink
	if err := userInterface.Output(msg); err != nil {
		logger.Debug().Err(err).Msg("failed to output status")
	}
}

//nolint:ireturn // workflow.Data is the framework's expected return type
func newWorkflowData(contentType string, data []byte) workflow.Data {
	return workflow.NewData(
		workflow.NewTypeIdentifier(WorkflowID, "redteam"),
		contentType,
		data,
	)
}
