// Package redteam implements the Snyk CLI workflow that runs a new Agent Red Team scan
// against a configured target (control server + target HTTP API) and returns the report.
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

	"github.com/rs/zerolog"
	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam/clireport"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam/htmlreport"
	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/helpers"
	"github.com/snyk/cli-extension-ai-redteam/internal/models"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

var WorkflowID = workflow.NewWorkflowIdentifier("redteam")

type (
	ControlServerFactory func(
		logger *zerolog.Logger, httpClient *http.Client, url, tenantID string,
	) controlserver.Client
	TargetFactory func(
		httpClient *http.Client, url string, headers map[string]string,
		bodyTemplate, responseSelector string,
	) target.Client
)

var DefaultSnykAPIFactory ControlServerFactory = func(
	logger *zerolog.Logger, httpClient *http.Client, url, tenantID string,
) controlserver.Client {
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
	flagset.String(utils.FlagJSONFileOutput, "", "Write the JSON report to the specified file path")
	flagset.Bool(utils.FlagJSON, false, "Output raw JSON instead of the styled CLI report")
	flagset.Bool(utils.FlagListGoals, false, "List all available attack goals and exit")
	flagset.Bool(utils.FlagListStrategies, false, "List all available attack strategies and exit")
	flagset.Bool(utils.FlagListProfiles, false, "List all available attack profiles and exit")
	flagset.String(utils.FlagGoals, "", "Comma-separated goals to test (e.g. system_prompt_extraction,pii_extraction)")
	flagset.String(utils.FlagProfile, "", "Attack profile to use (e.g. fast, security, safety)")
	flagset.String(utils.FlagTenantID, "", "Tenant ID (auto-discovered if not provided)")
	flagset.String(utils.FlagPurpose, "", "Intended purpose of the target (ground truth for the judge)")
	flagset.String(utils.FlagSystemPrompt, "", "Target system prompt (ground truth for prompt-extraction scoring)")
	flagset.String(utils.FlagTools, "", "Comma-separated tool names the target is configured with (ground truth)")
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

//nolint:gocyclo // inherent complexity of workflow orchestration
func RunRedTeamWorkflow(
	invocationCtx workflow.InvocationContext,
	controlServerFactory ControlServerFactory,
	targetFactory TargetFactory,
) ([]workflow.Data, error) {
	logger := invocationCtx.GetEnhancedLogger()
	config := invocationCtx.GetConfiguration()

	config.Set(configuration.RAW_CMD_ARGS, os.Args[1:])

	if err := utils.RejectOrgFlag(); err != nil {
		return nil, err //nolint:wrapcheck // already a catalog error
	}

	experimental := config.GetBool(utils.FlagExperimental)
	if !experimental {
		logger.Debug().Msg("Required experimental flag is not present")
		return nil, cli_errors.NewCommandIsExperimentalError("re-run with --experimental to use this command")
	}

	if err := utils.RequireAuth(config); err != nil {
		logger.Debug().Msg("No organization id is found.")
		return nil, err //nolint:wrapcheck // already a catalog error
	}

	if config.GetBool(utils.FlagReport) {
		data, meta, err := clireport.LoadReport()
		if err != nil {
			return nil, fmt.Errorf("failed to load report: %w", err)
		}
		if err := clireport.RunInteractive(data, meta); err != nil {
			report := clireport.Render(data, meta)
			return []workflow.Data{newWorkflowData(contentTypePlain, []byte(report))}, nil //nolint:nilerr // TUI failure is expected, fall back to static render
		}
		return []workflow.Data{newWorkflowData(contentTypePlain, []byte(""))}, nil
	}

	listGoals := config.GetBool(utils.FlagListGoals)
	listStrategies := config.GetBool(utils.FlagListStrategies)
	listProfiles := config.GetBool(utils.FlagListProfiles)
	if listGoals || listStrategies || listProfiles {
		httpClient := invocationCtx.GetNetworkAccess().GetHttpClient()
		return handleListFlags(config, controlServerFactory, logger, httpClient, listGoals, listStrategies, listProfiles)
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
		return nil, err //nolint:wrapcheck // RedTeamError from helpers
	}

	targetHTTPClient := &http.Client{Timeout: rtConfig.TargetHTTPTimeout()}
	controlServerHTTPClient := invocationCtx.GetNetworkAccess().GetHttpClient()
	controlServerHTTPClient.Timeout = controlserver.DefaultClientTimeout
	controlServerURL := config.GetString(configuration.API_URL)

	controlServerClient := controlServerFactory(logger, controlServerHTTPClient, controlServerURL, tenantID)

	profileName, err := resolveAttacks(config, controlServerClient, rtConfig)
	if err != nil {
		return nil, err
	}

	configPath := config.GetString(utils.FlagConfig)
	if configPath == "" {
		if _, statErr := os.Stat("redteam.yaml"); statErr == nil {
			configPath = "redteam.yaml"
		}
	}

	targetClient := targetFactory(
		targetHTTPClient,
		rtConfig.Target.Settings.URL,
		rtConfig.HeadersMap(),
		rtConfig.Target.Settings.RequestBodyTemplate,
		rtConfig.Target.Settings.ResponseSelector,
	)

	results, scanErr := runClientDrivenScan(invocationCtx, controlServerClient, targetClient, rtConfig, profileName, configPath)
	if scanErr != nil {
		return nil, scanErr
	}

	output, htmlErr := htmlreport.ProcessResults(logger, config, results)
	if htmlErr != nil {
		return nil, htmlErr //nolint:wrapcheck // RedTeamError from htmlreport
	}

	returnJSON := config.GetBool(utils.FlagJSON) || config.GetString(utils.FlagJSONFileOutput) != ""
	returnHTML := config.GetBool(utils.FlagHTML)
	//nolint:nestif // sequential branching for TUI/static report fallback
	if !returnJSON && !returnHTML && len(results) > 0 {
		reportData, parseErr := parseReportForTUI(results, config)
		if parseErr == nil && reportData != nil {
			meta := clireport.ScanMeta{
				TargetURL:  rtConfig.Target.Settings.URL,
				Goals:      rtConfig.UniqueGoals(),
				Strategies: rtConfig.UniqueStrategies(),
			}
			// Save report for later re-display via --report.
			if saveErr := clireport.SaveReport(reportData, meta); saveErr != nil {
				logger.Debug().Err(saveErr).Msg("failed to save report for --report flag")
			}
			if err := clireport.RunInteractive(reportData, meta); err != nil {
				// Fallback to static report if TUI fails (e.g. piped output).
				report := clireport.Render(reportData, meta)
				return []workflow.Data{newWorkflowData(contentTypePlain, []byte(report))}, nil //nolint:nilerr // TUI failure is expected, fall back to static render
			}
			return []workflow.Data{newWorkflowData(contentTypePlain, []byte(""))}, nil
		}
	}

	return output, nil
}

func parseReportForTUI(results []workflow.Data, config configuration.Configuration) (*models.ScanReport, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no results")
	}
	payload, ok := results[0].GetPayload().([]byte)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}
	var data models.ScanReport
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("failed to parse report JSON: %w", err)
	}
	_ = config // reserved for future use (e.g. building summary from status)
	return &data, nil
}

func handleListFlags(
	config configuration.Configuration,
	controlServerFactory ControlServerFactory,
	logger *zerolog.Logger,
	httpClient *http.Client,
	listGoals, listStrategies, listProfiles bool,
) ([]workflow.Data, error) {
	ctx := context.Background()
	httpClient.Timeout = controlserver.DefaultClientTimeout
	snykAPIURL := config.GetString(configuration.API_URL)
	controlServerClient := controlServerFactory(logger, httpClient, snykAPIURL, "")

	var lines []string
	if listGoals {
		goals, err := controlServerClient.ListGoals(ctx)
		if err != nil {
			return nil, err //nolint:wrapcheck // RedTeamError from controlserver
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
			return nil, err //nolint:wrapcheck // RedTeamError from controlserver
		}
		sort.Slice(strategies, func(i, j int) bool { return strategies[i].DisplayOrder < strategies[j].DisplayOrder })
		lines = append(lines, "Available strategies:", "")
		lines = appendEnumTable(lines, strategies)
	}
	if listProfiles {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		profiles, err := controlServerClient.ListProfiles(ctx)
		if err != nil {
			return nil, err //nolint:wrapcheck // RedTeamError from controlserver
		}
		lines = append(lines, "Available profiles:", "")
		lines = appendProfileTable(lines, profiles)
	}

	output := strings.Join(lines, "\n") + "\n"
	return []workflow.Data{newWorkflowData(contentTypePlain, []byte(output))}, nil
}

func getGoalsFlag(config configuration.Configuration) []string {
	raw := config.GetString(utils.FlagGoals)
	if raw == "" {
		return nil
	}
	var goals []string
	for _, g := range strings.Split(raw, ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			goals = append(goals, g)
		}
	}
	return goals
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

func appendProfileTable(lines []string, profiles []controlserver.ProfileResponse) []string {
	idWidth := len("ID")
	nameWidth := len("NAME")
	for _, p := range profiles {
		if len(p.ID) > idWidth {
			idWidth = len(p.ID)
		}
		if len(p.Name) > nameWidth {
			nameWidth = len(p.Name)
		}
	}
	idWidth += 2
	nameWidth += 2

	lines = append(lines, fmt.Sprintf("  %-*s  %-*s  %s", idWidth, "ID", nameWidth, "NAME", "ATTACKS"))
	for _, p := range profiles {
		lines = append(lines, fmt.Sprintf("  %-*s  %-*s  %d", idWidth, p.ID, nameWidth, p.Name, len(p.Entries)))
	}
	return lines
}

func runClientDrivenScan(
	invocationCtx workflow.InvocationContext,
	csClient controlserver.Client,
	targetClient target.Client,
	rtConfig *Config,
	profileName string,
	configPath string,
) ([]workflow.Data, error) {
	logger := invocationCtx.GetEnhancedLogger()
	userInterface := invocationCtx.GetUserInterface()
	ctx := context.Background()

	scanID, err := csClient.CreateScan(ctx, rtConfig.ToCreateScanRequest())
	if err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from controlserver
	}
	logger.Info().Str("scanID", scanID).Msg("scan created successfully")

	// --- Logo + Banner ---
	printLogo(userInterface)
	printBanner(userInterface, bannerParams{
		ScanID:      scanID,
		TargetURL:   rtConfig.Target.Settings.URL,
		ProfileName: profileName,
		Goals:       rtConfig.UniqueGoals(),
		Strategies:  rtConfig.UniqueStrategies(),
		ConfigPath:  configPath,
	})

	// --- Live progress UI ---
	progress := newProgressUI(os.Stderr, supportsColor())
	progress.Start()

	// Background goroutine polls status every 500ms so probe counters tick up live.
	pollCtx, pollCancel := context.WithCancel(ctx)
	defer pollCancel()
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-pollCtx.Done():
				return
			case <-ticker.C:
				if status, err := csClient.GetStatus(pollCtx, scanID); err == nil {
					progress.Update(status)
				}
			}
		}
	}()

	var responses []controlserver.ChatResponse
	for {
		chats, nextErr := csClient.NextChats(ctx, scanID, responses)
		if nextErr != nil {
			pollCancel()
			progress.Stop()
			return nil, nextErr //nolint:wrapcheck // RedTeamError from controlserver
		}
		if len(chats) == 0 {
			break
		}

		responses = make([]controlserver.ChatResponse, 0, len(chats))
		for _, chat := range chats {
			resp, tgtErr := targetClient.SendPrompt(ctx, chat.Prompt)
			if tgtErr != nil {
				if errors.Is(tgtErr, target.ErrCircuitOpen) {
					pollCancel()
					progress.Stop()
					return nil, redteam_errors.NewNetworkError(fmt.Sprintf("aborting scan: %s", tgtErr))
				}
				logger.Warn().Err(tgtErr).Str("scanID", scanID).Str("chatID", chat.ChatID).Msg("The scan target returned an error")
				resp = fmt.Sprintf("[error: %s]", tgtErr.Error())
			}
			responses = append(responses, controlserver.ChatResponse{
				Seq:      chat.Seq,
				Response: resp,
			})
			progress.IncrementSent()
		}
	}

	// --- Completion ---
	pollCancel()
	progress.Stop()
	finalStatus, statusErr := csClient.GetStatus(ctx, scanID)
	if statusErr != nil {
		logger.Debug().Err(statusErr).Msg("failed to get final status")
	}
	progress.Finish(finalStatus)

	reportJSON, reportErr := csClient.GetReport(ctx, scanID)
	if reportErr != nil {
		return nil, reportErr //nolint:wrapcheck // RedTeamError from controlserver
	}

	return []workflow.Data{newWorkflowData("application/json", reportJSON)}, nil
}

//nolint:ireturn // workflow.Data is the framework's expected return type
func newWorkflowData(contentType string, data []byte) workflow.Data {
	return workflow.NewData(
		workflow.NewTypeIdentifier(WorkflowID, "redteam"),
		contentType,
		data,
	)
}
