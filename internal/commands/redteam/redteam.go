package redteam

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/ui"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam/htmlreport"
	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/helpers"
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
		bodyTemplate, responseSelector string, opts ...target.ClientOption,
	) target.Client
)

var DefaultSnykAPIFactory ControlServerFactory = func(
	logger *zerolog.Logger, httpClient *http.Client, url, tenantID string,
) controlserver.Client {
	return controlserver.NewClient(logger, httpClient, url, tenantID)
}

var DefaultTargetFactory TargetFactory = func(
	httpClient *http.Client, url string, headers map[string]string,
	bodyTemplate, responseSelector string, opts ...target.ClientOption,
) target.Client {
	return target.NewHTTPClient(httpClient, url, headers, bodyTemplate, responseSelector, opts...)
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
	flagset.Bool(utils.FlagListGoals, false, "List all available attack goals and exit")
	flagset.Bool(utils.FlagListStrategies, false, "List all available attack strategies and exit")
	flagset.Bool(utils.FlagListProfiles, false, "List all available attack profiles and exit")
	flagset.String(utils.FlagGoals, "", "Comma-separated goals to test (e.g. system_prompt_extraction,pii_extraction)")
	flagset.String(utils.FlagProfile, "", "Attack profile to use (e.g. fast, security, safety)")
	flagset.String(utils.FlagTenantID, "", "Tenant ID (auto-discovered if not provided)")
	flagset.String(utils.FlagPurpose, "", "Intended purpose of the target (ground truth for the judge)")
	flagset.String(utils.FlagSystemPrompt, "", "Target system prompt (ground truth for prompt-extraction scoring)")
	flagset.String(utils.FlagTools, "", "Comma-separated tool names the target is configured with (ground truth)")
	flagset.Int(utils.FlagConcurrency, 0, "Number of chat sessions to run in parallel (default: 1)")

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
		return nil, cli_errors.NewCommandIsExperimentalError("re-run with --experimental to use this command")
	}

	if err := utils.RequireAuth(config); err != nil {
		logger.Debug().Msg("No organization id is found.")
		return nil, err //nolint:wrapcheck // already a catalog error
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

	controlServerHTTPClient := invocationCtx.GetNetworkAccess().GetHttpClient()
	controlServerHTTPClient.Timeout = 60 * time.Second
	controlServerURL := config.GetString(configuration.API_URL)

	controlServerClient := controlServerFactory(logger, controlServerHTTPClient, controlServerURL, tenantID)

	profileName, err := resolveAttacks(config, controlServerClient, rtConfig)
	if err != nil {
		return nil, err
	}

	userInterface := invocationCtx.GetUserInterface()
	displayBanner(userInterface, rtConfig, profileName)

	results, scanErr := runClientDrivenScan(invocationCtx, controlServerClient, targetFactory, rtConfig)
	if scanErr != nil {
		return nil, scanErr
	}

	output, htmlErr := htmlreport.ProcessResults(logger, config, results)
	if htmlErr != nil {
		return nil, htmlErr //nolint:wrapcheck // RedTeamError from htmlreport
	}
	return output, nil
}

func handleListFlags(
	config configuration.Configuration,
	controlServerFactory ControlServerFactory,
	logger *zerolog.Logger,
	httpClient *http.Client,
	listGoals, listStrategies, listProfiles bool,
) ([]workflow.Data, error) {
	ctx := context.Background()
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

// sessionManager tracks per-chat target clients.
type sessionManager struct {
	factory       func(url string) target.Client
	urlCommand    *utils.ExternalCommand
	sessions      map[string]target.Client
	defaultClient target.Client
	logger        *zerolog.Logger
}

func newSessionManager(
	logger *zerolog.Logger,
	rtConfig *Config,
	targetFactory TargetFactory,
) *sessionManager {
	// When target_command is set, it handles the entire interaction — no HTTP client needed.
	if rtConfig.Target.Settings.TargetCommand != nil {
		return &sessionManager{
			defaultClient: target.NewExternalClient(rtConfig.Target.Settings.TargetCommand),
			sessions:      make(map[string]target.Client),
			logger:        logger,
		}
	}

	var targetOpts []target.ClientOption
	if rtConfig.Target.Settings.RequestCommand != nil {
		targetOpts = append(targetOpts, target.WithRequestCommand(rtConfig.Target.Settings.RequestCommand))
	}
	if rtConfig.Target.Settings.ResponseCommand != nil {
		targetOpts = append(targetOpts, target.WithResponseCommand(rtConfig.Target.Settings.ResponseCommand))
	}
	targetHTTPClient := &http.Client{Timeout: target.DefaultTimeout}
	headers := rtConfig.HeadersMap()
	bodyTmpl := rtConfig.Target.Settings.RequestBodyTemplate
	respSel := rtConfig.Target.Settings.ResponseSelector

	factory := func(url string) target.Client {
		return targetFactory(targetHTTPClient, url, headers, bodyTmpl, respSel, targetOpts...)
	}

	sm := &sessionManager{
		factory:    factory,
		urlCommand: rtConfig.Target.Settings.URLCommand,
		sessions:   make(map[string]target.Client),
		logger:     logger,
	}
	if sm.urlCommand == nil {
		sm.defaultClient = factory(rtConfig.Target.Settings.URL)
	}
	return sm
}

//nolint:ireturn // target.Client is the expected interface for polymorphic target clients
func (sm *sessionManager) getClient(ctx context.Context, chatID string) (target.Client, error) {
	if sm.urlCommand == nil {
		return sm.defaultClient, nil
	}
	if c, ok := sm.sessions[chatID]; ok {
		return c, nil
	}
	resolved, err := sm.urlCommand.Run(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("url_command for chat %s: %w", chatID, err)
	}
	sm.logger.Debug().Str("chatID", chatID).Str("url", resolved).Msg("new session URL")
	c := sm.factory(resolved)
	sm.sessions[chatID] = c
	return c, nil
}

func (sm *sessionManager) cleanupSessions(chats []controlserver.ChatPrompt) {
	if sm.urlCommand == nil {
		return
	}
	active := make(map[string]struct{}, len(chats))
	for _, chat := range chats {
		active[chat.ChatID] = struct{}{}
	}
	for chatID := range sm.sessions {
		if _, ok := active[chatID]; !ok {
			delete(sm.sessions, chatID)
		}
	}
}

func (sm *sessionManager) resolveClients(ctx context.Context, chats []controlserver.ChatPrompt) ([]target.Client, error) {
	clients := make([]target.Client, len(chats))
	for i, chat := range chats {
		c, err := sm.getClient(ctx, chat.ChatID)
		if err != nil {
			return nil, err
		}
		clients[i] = c
	}
	return clients, nil
}

type promptResult struct {
	seq      int
	response string
}

func sendPromptsConcurrently(
	ctx context.Context,
	logger *zerolog.Logger,
	scanID string,
	chats []controlserver.ChatPrompt,
	clients []target.Client,
	concurrency int,
) ([]promptResult, error) {
	results := make([]promptResult, len(chats))
	var circuitErr atomic.Value
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, chat := range chats {
		wg.Add(1)
		sem <- struct{}{} // acquire
		go func(idx int, c controlserver.ChatPrompt, tc target.Client) {
			defer wg.Done()
			defer func() { <-sem }() // release

			promptCtx := target.WithChatContext(ctx, target.ChatContext{ChatID: c.ChatID, Seq: c.Seq})
			resp, tgtErr := tc.SendPrompt(promptCtx, c.Prompt)
			if tgtErr != nil {
				if errors.Is(tgtErr, target.ErrCircuitOpen) {
					circuitErr.Store(tgtErr)
				}
				logger.Warn().Err(tgtErr).Str("scanID", scanID).Str("chatID", c.ChatID).Msg("The scan target returned an error")
				resp = fmt.Sprintf("[error: %s]", tgtErr.Error())
			}
			results[idx] = promptResult{seq: c.Seq, response: resp}
		}(i, chat, clients[i])
	}
	wg.Wait()

	if stored := circuitErr.Load(); stored != nil {
		return nil, fmt.Errorf("aborting scan: %s", stored) //nolint:err113 // dynamic circuit-breaker message
	}
	return results, nil
}

func runClientDrivenScan(
	invocationCtx workflow.InvocationContext,
	csClient controlserver.Client,
	targetFactory TargetFactory,
	rtConfig *Config,
) ([]workflow.Data, error) {
	logger := invocationCtx.GetEnhancedLogger()
	userInterface := invocationCtx.GetUserInterface()
	ctx := context.Background()

	sm := newSessionManager(logger, rtConfig, targetFactory)

	scanID, err := csClient.CreateScan(ctx, rtConfig.ToCreateScanRequest())
	if err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from controlserver
	}
	logger.Info().Str("scanID", scanID).Msg("scan created successfully")

	progressBar := userInterface.NewProgressBar()
	progressBar.SetTitle(fmt.Sprintf("Scanning %s...", rtConfig.Target.Name))
	_ = progressBar.UpdateProgress(ui.InfiniteProgress) //nolint:errcheck // best-effort UI
	defer func() { _ = progressBar.Clear() }()          //nolint:errcheck // best-effort UI

	var responses []controlserver.ChatResponse
	for {
		chats, nextErr := csClient.NextChats(ctx, scanID, responses)
		if nextErr != nil {
			return nil, nextErr //nolint:wrapcheck // RedTeamError from controlserver
		}
		if len(chats) == 0 {
			break
		}

		sm.cleanupSessions(chats)

		chatClients, clientErr := sm.resolveClients(ctx, chats)
		if clientErr != nil {
			return nil, redteam_errors.NewNetworkError(clientErr.Error())
		}

		results, sendErr := sendPromptsConcurrently(ctx, logger, scanID, chats, chatClients, rtConfig.Concurrency)
		if sendErr != nil {
			return nil, redteam_errors.NewNetworkError(sendErr.Error())
		}

		responses = make([]controlserver.ChatResponse, 0, len(chats))
		for _, r := range results {
			responses = append(responses, controlserver.ChatResponse{
				Seq:      r.seq,
				Response: r.response,
			})
		}

		updateProgress(ctx, csClient, scanID, progressBar, userInterface, logger)
	}

	progressBar.SetTitle("Scan completed")
	_ = progressBar.UpdateProgress(1.0) //nolint:errcheck // best-effort UI

	status, statusErr := csClient.GetStatus(ctx, scanID)
	if statusErr != nil {
		logger.Debug().Err(statusErr).Msg("failed to get final status")
	}

	outputStatus(userInterface, logger, status)

	reportJSON, reportErr := csClient.GetReport(ctx, scanID)
	if reportErr != nil {
		return nil, reportErr //nolint:wrapcheck // RedTeamError from controlserver
	}

	return []workflow.Data{newWorkflowData("application/json", reportJSON)}, nil
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
		_ = progressBar.UpdateProgress(float64(status.Completed) / float64(status.TotalChats)) //nolint:errcheck // best-effort UI
	}
}

func outputStatus(userInterface ui.UserInterface, logger *zerolog.Logger, status *controlserver.ScanStatus) {
	if status == nil {
		return
	}
	msg := fmt.Sprintf("\nScan complete: %d/%d chats | %d successful | %d failed",
		status.Completed, status.TotalChats, status.Successful, status.Failed)
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
