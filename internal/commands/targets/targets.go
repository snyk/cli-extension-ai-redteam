package targets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/rs/zerolog"
	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/helpers"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
	"github.com/snyk/cli-extension-ai-redteam/internal/wizard"
)

const targetsWorkflowName = "redteam.targets"

var (
	TargetsWorkflowID   = workflow.NewWorkflowIdentifier(targetsWorkflowName)
	targetsWorkflowType = workflow.NewTypeIdentifier(TargetsWorkflowID, targetsWorkflowName)
)

const (
	flagPort   = "port"
	flagOutput = "output"
)

type ControlServerFactory func(
	logger *zerolog.Logger, httpClient *http.Client, url, tenantID string,
) controlserver.Client

func RegisterTargetsWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-targets", pflag.ExitOnError)
	flagset.Bool(utils.FlagExperimental, false,
		"This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagTenantID, "", "Tenant ID (auto-discovered if not provided)")
	flagset.String(utils.FlagConfig, "", "Path to the red team configuration file (default: redteam.yaml)")
	flagset.String(utils.FlagTargetName, "", "Target name override (for save sub-action)")
	flagset.Int(flagPort, 8484, "Port for the setup wizard web server (for get sub-action)")
	flagset.String(flagOutput, "", "Write the fetched target config to a file instead of opening the wizard")

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(TargetsWorkflowID, cfg, targetsWorkflow); err != nil {
		return fmt.Errorf("error while registering targets workflow: %w", err)
	}
	return nil
}

func targetsWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	factory := ControlServerFactory(
		func(logger *zerolog.Logger, httpClient *http.Client, url, tenantID string) controlserver.Client {
			return controlserver.NewClient(logger, httpClient, url, tenantID)
		})
	return RunTargetsWorkflow(invocationCtx, factory)
}

func RunTargetsWorkflow(
	invocationCtx workflow.InvocationContext,
	controlServerFactory ControlServerFactory,
) ([]workflow.Data, error) {
	config := invocationCtx.GetConfiguration()
	logger := invocationCtx.GetEnhancedLogger()

	if err := utils.RejectOrgFlag(); err != nil {
		return nil, err //nolint:wrapcheck // already a catalog error
	}

	if err := utils.RequireAuth(config); err != nil {
		return nil, err //nolint:wrapcheck // already a catalog error
	}

	if !config.GetBool(utils.FlagExperimental) {
		return nil, cli_errors.NewCommandIsExperimentalError("re-run with --experimental to use this command")
	}

	tenantID, err := helpers.GetTenantID(invocationCtx, config.GetString(utils.FlagTenantID))
	if err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from helpers
	}

	httpClient := invocationCtx.GetNetworkAccess().GetHttpClient()
	httpClient.Timeout = controlserver.DefaultClientTimeout
	apiURL := config.GetString(configuration.API_URL)
	csClient := controlServerFactory(logger, httpClient, apiURL, tenantID)

	action, arg := parseSubAction()
	ctx := context.Background()

	switch action {
	case "list":
		return handleList(ctx, csClient)
	case "get":
		if arg == "" {
			return nil, redteam_errors.NewBadRequestError("usage: snyk redteam targets get <name-or-id>")
		}
		return handleGet(ctx, invocationCtx, csClient, arg, config)
	case "save":
		return handleSave(ctx, csClient, config)
	case "delete":
		if arg == "" {
			return nil, redteam_errors.NewBadRequestError("usage: snyk redteam targets delete <name-or-id>")
		}
		return handleDelete(ctx, csClient, arg)
	default:
		return nil, redteam_errors.NewBadRequestError(
			"usage: snyk redteam targets [list|get|save|delete]",
		)
	}
}

// parseSubAction extracts the sub-action and optional argument from os.Args.
// Expected patterns: "snyk redteam targets list", "snyk redteam targets get <arg>", etc.
func parseSubAction() (action, arg string) {
	args := os.Args[1:]

	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			if strings.Contains(a, "=") {
				continue
			}
			// Skip flags that take a value argument (known value flags).
			switch a {
			case "--tenant-id", "--config", "--target-name", "--port", "--output":
				i++
			}
			continue
		}
		positional = append(positional, a)
	}

	// positional: ["redteam", "targets", "<action>", "<arg>"]
	idx := -1
	for i, p := range positional {
		if p == "targets" {
			idx = i
			break
		}
	}
	if idx < 0 || idx+1 >= len(positional) {
		return "", ""
	}
	action = positional[idx+1]
	if idx+2 < len(positional) {
		arg = positional[idx+2]
	}
	return action, arg
}

func handleList(ctx context.Context, client controlserver.Client) ([]workflow.Data, error) {
	items, err := client.ListTargets(ctx)
	if err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from controlserver
	}

	if len(items) == 0 {
		return textOutput("No saved targets found.\n"), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%-30s %-38s %s\n", "NAME", "ID", "UPDATED")
	for _, item := range items {
		fmt.Fprintf(&sb, "%-30s %-38s %s\n", item.Name, item.ID, item.UpdatedAt)
	}
	return textOutput(sb.String()), nil
}

func handleGet(
	ctx context.Context,
	invocationCtx workflow.InvocationContext,
	client controlserver.Client,
	nameOrID string,
	config configuration.Configuration,
) ([]workflow.Data, error) {
	targetID, err := resolveTargetID(ctx, client, nameOrID)
	if err != nil {
		return nil, err
	}

	resp, err := client.GetTarget(ctx, targetID)
	if err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from controlserver
	}

	rtConfig, err := configFromMap(resp.Config)
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("failed to parse target config: %s", err))
	}

	if rtConfig.Target.Name == "" {
		rtConfig.Target.Name = resp.Name
	}

	yamlBytes, err := yaml.Marshal(rtConfig)
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("marshal config to YAML: %s", err))
	}

	if outputPath := config.GetString(flagOutput); outputPath != "" {
		if writeErr := os.WriteFile(outputPath, yamlBytes, 0o600); writeErr != nil {
			return nil, redteam_errors.NewInternalError(fmt.Sprintf("write config to %s: %s", outputPath, writeErr))
		}
		msg := fmt.Sprintf("Target %q config written to %s\n", resp.Name, outputPath)
		return textOutput(msg), nil
	}

	tmpDir, err := os.MkdirTemp("", "redteam-target-*")
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("create temp dir: %s", err))
	}

	configPath := tmpDir + "/redteam.yaml"
	if err := os.WriteFile(configPath, yamlBytes, 0o600); err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("write temp config: %s", err))
	}

	port := config.GetInt(flagPort)
	apiURL := config.GetString(configuration.API_URL)
	logger := zerolog.Nop()
	httpClient := invocationCtx.GetNetworkAccess().GetHttpClient()
	httpClient.Timeout = controlserver.DefaultClientTimeout
	csClient := controlserver.NewClient(&logger, httpClient, apiURL, "")

	userInterface := invocationCtx.GetUserInterface()
	server := wizard.NewServer(port, configPath, rtConfig, csClient, userInterface)
	if err := server.Start(); err != nil { //nolint:contextcheck // Start does not accept context
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("setup wizard error: %s", err))
	}

	return nil, nil
}

func handleSave(
	ctx context.Context,
	client controlserver.Client,
	config configuration.Configuration,
) ([]workflow.Data, error) {
	configPath := config.GetString(utils.FlagConfig)
	if configPath == "" {
		configPath = "redteam.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, redteam_errors.NewConfigValidationError(
			fmt.Sprintf("failed to read config file %s: %s", configPath, err),
		)
	}

	var rtConfig redteam.Config
	if unmarshalErr := yaml.Unmarshal(data, &rtConfig); unmarshalErr != nil {
		return nil, redteam_errors.NewConfigValidationError(
			fmt.Sprintf("failed to parse %s: %s", configPath, unmarshalErr),
		)
	}

	name := rtConfig.Target.Name
	if flagName := config.GetString(utils.FlagTargetName); flagName != "" {
		name = flagName
	}
	if name == "" {
		return nil, redteam_errors.NewBadRequestError(
			"target name is required: set target.name in config or use --name",
		)
	}

	configMap, err := structToMap(&rtConfig)
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("serialize config: %s", err))
	}

	stripHeaders(configMap)

	resp, err := client.CreateTarget(ctx, &controlserver.TargetCreateRequest{
		Name:   name,
		Config: configMap,
	})
	if err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from controlserver
	}

	msg := fmt.Sprintf("Target %q saved (id: %s)\n", resp.Name, resp.ID)
	return textOutput(msg), nil
}

func handleDelete(
	ctx context.Context,
	client controlserver.Client,
	nameOrID string,
) ([]workflow.Data, error) {
	targetID, err := resolveTargetID(ctx, client, nameOrID)
	if err != nil {
		return nil, err
	}

	if err := client.DeleteTarget(ctx, targetID); err != nil {
		return nil, err //nolint:wrapcheck // RedTeamError from controlserver
	}

	msg := fmt.Sprintf("Target %q deleted\n", nameOrID)
	return textOutput(msg), nil
}

// resolveTargetID resolves a name-or-id string to a UUID target ID.
// If nameOrID looks like a UUID it is returned as-is; otherwise the target
// list is fetched and the first matching name is used.
func resolveTargetID(ctx context.Context, client controlserver.Client, nameOrID string) (string, error) {
	if looksLikeUUID(nameOrID) {
		return nameOrID, nil
	}

	items, err := client.ListTargets(ctx)
	if err != nil {
		return "", err //nolint:wrapcheck // RedTeamError from controlserver
	}

	for _, item := range items {
		if item.Name == nameOrID {
			return item.ID, nil
		}
	}

	return "", redteam_errors.NewNotFoundError(fmt.Sprintf("target %q not found", nameOrID))
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func configFromMap(m map[string]any) (*redteam.Config, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal config map: %w", err)
	}
	var cfg redteam.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

func structToMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return m, nil
}

// stripHeaders removes target.settings.headers from the config map before persisting.
func stripHeaders(configMap map[string]any) {
	t, ok := configMap["target"]
	if !ok {
		return
	}
	tm, ok := t.(map[string]any)
	if !ok {
		return
	}
	s, ok := tm["settings"]
	if !ok {
		return
	}
	sm, ok := s.(map[string]any)
	if !ok {
		return
	}
	delete(sm, "headers")
}

func textOutput(s string) []workflow.Data {
	return []workflow.Data{
		workflow.NewData(targetsWorkflowType, "text/plain", []byte(s)),
	}
}
