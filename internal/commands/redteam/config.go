package redteam

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/jmespath/go-jmespath"
	"github.com/rs/zerolog"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"gopkg.in/yaml.v3"

	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

const (
	defaultRequestBodyTemplate = `{"message": "{{prompt}}"}`
	defaultTargetType          = "http"
	contentTypePlain           = "text/plain"
)

const defaultProfileID = "fast"

type Config struct {
	Target      ConfigTarget                `yaml:"target" json:"target"`
	Goals       []string                    `yaml:"goals" json:"goals"`
	Attacks     []controlserver.AttackEntry `yaml:"attacks" json:"attacks,omitempty"`
	Concurrency int                         `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`
}

type ConfigTarget struct {
	Name     string         `yaml:"name" json:"name"`
	Type     string         `yaml:"type" json:"type"`
	Context  ConfigContext  `yaml:"context" json:"context"`
	Settings ConfigSettings `yaml:"settings" json:"settings"`
}

type ConfigContext struct {
	Purpose     string            `yaml:"purpose" json:"purpose"`
	GroundTruth ConfigGroundTruth `yaml:"ground_truth" json:"ground_truth,omitempty"`
}

type ConfigGroundTruth struct {
	SystemPrompt string   `yaml:"system_prompt" json:"system_prompt,omitempty"`
	Tools        []string `yaml:"tools" json:"tools,omitempty"`
}

type ConfigSettings struct {
	URL                 string                 `yaml:"url" json:"url"`
	URLCommand          *utils.ExternalCommand `yaml:"url_command,omitempty" json:"url_command,omitempty"`
	Headers             []ConfigHeader         `yaml:"headers,omitempty" json:"headers,omitempty"`
	ResponseSelector    string                 `yaml:"response_selector" json:"response_selector"`
	ResponseCommand     *utils.ExternalCommand `yaml:"response_command,omitempty" json:"response_command,omitempty"`
	RequestBodyTemplate string                 `yaml:"request_body_template" json:"request_body_template"`
	RequestCommand      *utils.ExternalCommand `yaml:"request_command,omitempty" json:"request_command,omitempty"`
	TargetCommand       *utils.ExternalCommand `yaml:"target_command,omitempty" json:"target_command,omitempty"`
}

type ConfigHeader struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

func LoadAndValidateConfig(
	logger *zerolog.Logger, config configuration.Configuration,
) (*Config, []workflow.Data, error) {
	rtConfig, earlyReturn := loadConfigFromFile(logger, config)
	if earlyReturn != nil {
		return nil, earlyReturn, nil
	}

	applyTargetURLOverride(config, rtConfig)
	applyFlagOverrides(config, rtConfig)

	if err := resolveURLCommand(logger, rtConfig); err != nil {
		return nil, nil, err
	}

	applyDefaults(rtConfig)

	if err := ValidateConfig(rtConfig); err != nil {
		return nil, nil, err
	}

	return rtConfig, nil, nil
}

func loadConfigFromFile(logger *zerolog.Logger, config configuration.Configuration) (*Config, []workflow.Data) {
	targetURL := config.GetString(utils.FlagTargetURL)
	configPath := config.GetString(utils.FlagConfig)

	var rtConfig Config

	hasConfigFile := false
	if configPath != "" {
		hasConfigFile = true
	} else {
		configPath = "redteam.yaml"
		if _, err := os.Stat(configPath); err == nil {
			hasConfigFile = true
		}
	}

	if hasConfigFile {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			message := fmt.Sprintf("Configuration file not found: %s", configPath)
			return nil, []workflow.Data{newWorkflowData(contentTypePlain, []byte(message))}
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			logger.Debug().Err(err).Msg("error reading config file")
			return nil, []workflow.Data{newWorkflowData(contentTypePlain, []byte(getInvalidConfigMessage()))}
		}

		if err := yaml.Unmarshal(data, &rtConfig); err != nil {
			logger.Debug().Err(err).Msg("error unmarshaling config")
			return nil, []workflow.Data{newWorkflowData(contentTypePlain, []byte(getInvalidConfigMessage()))}
		}
	} else if targetURL == "" {
		message := `No configuration found. Either:
  - Create a redteam.yaml in the current directory
  - Use --config to specify a config file
  - Use --target-url to scan a target directly`
		return nil, []workflow.Data{newWorkflowData(contentTypePlain, []byte(message))}
	}

	return &rtConfig, nil
}

func resolveURLCommand(logger *zerolog.Logger, cfg *Config) error {
	cmd := cfg.Target.Settings.URLCommand
	if cmd == nil {
		return nil
	}
	if err := cmd.Validate("url_command"); err != nil {
		return redteam_errors.NewConfigValidationError(fmt.Sprintf("invalid configuration:\n  - %s", err.Error()))
	}

	logger.Debug().Str("binary", cmd.Binary).Msg("resolving target URL via url_command")

	resolved, err := cmd.Run(context.Background(), "")
	if err != nil {
		return redteam_errors.NewNetworkError(fmt.Sprintf("url_command failed: %s", err))
	}
	if resolved == "" {
		return redteam_errors.NewConfigValidationError("url_command returned empty output; expected a target URL")
	}

	logger.Debug().Str("resolved_url", resolved).Msg("resolved target URL from url_command")
	cfg.Target.Settings.URL = resolved
	if cfg.Target.Name == "" {
		cfg.Target.Name = resolved
	}
	return nil
}

func applyTargetURLOverride(config configuration.Configuration, rtConfig *Config) {
	targetURL := config.GetString(utils.FlagTargetURL)
	if targetURL == "" {
		return
	}
	rtConfig.Target.Settings.URL = targetURL
	if rtConfig.Target.Name == "" {
		rtConfig.Target.Name = targetURL
	}
}

func applyFlagOverrides(config configuration.Configuration, rtConfig *Config) {
	if v := config.GetInt(utils.FlagConcurrency); v > 0 {
		rtConfig.Concurrency = v
	}
	if v := config.GetString(utils.FlagRequestBodyTmpl); v != "" {
		rtConfig.Target.Settings.RequestBodyTemplate = v
	}
	if v := config.GetString(utils.FlagResponseSelector); v != "" {
		rtConfig.Target.Settings.ResponseSelector = v
	}
	if headers := parseHeaderFlags(config); len(headers) > 0 {
		rtConfig.Target.Settings.Headers = append(rtConfig.Target.Settings.Headers, headers...)
	}
	if v := config.GetString(utils.FlagPurpose); v != "" {
		rtConfig.Target.Context.Purpose = v
	}
	if v := config.GetString(utils.FlagSystemPrompt); v != "" {
		rtConfig.Target.Context.GroundTruth.SystemPrompt = v
	}
	if tools := getToolsFlags(config); len(tools) > 0 {
		rtConfig.Target.Context.GroundTruth.Tools = tools
	}
}

// ToCreateScanRequest builds the control server CreateScan request from config.
func (cfg *Config) ToCreateScanRequest() *controlserver.CreateScanRequest {
	attacks := cfg.Attacks
	if len(attacks) == 0 {
		attacks = make([]controlserver.AttackEntry, 0, len(cfg.Goals))
		for _, g := range cfg.Goals {
			attacks = append(attacks, controlserver.AttackEntry{Goal: g})
		}
	}
	req := &controlserver.CreateScanRequest{
		Attacks:     attacks,
		Purpose:     cfg.Target.Context.Purpose,
		GroundTruth: buildGroundTruthFromConfig(&cfg.Target.Context.GroundTruth),
		TargetURL:   cfg.Target.Settings.URL,
		Concurrency: cfg.Concurrency,
	}
	return req
}

func buildGroundTruthFromConfig(gt *ConfigGroundTruth) *controlserver.GroundTruth {
	if gt.SystemPrompt == "" && len(gt.Tools) == 0 {
		return nil
	}
	return &controlserver.GroundTruth{
		SystemPrompt: gt.SystemPrompt,
		Tools:        strings.Join(gt.Tools, ", "),
	}
}

func ValidateConfig(cfg *Config) error {
	var errs []string

	// When target_command handles the entire interaction, only validate the command itself.
	if cfg.Target.Settings.TargetCommand != nil {
		if err := cfg.Target.Settings.TargetCommand.Validate("target_command"); err != nil {
			errs = append(errs, err.Error())
		}
	} else {
		errs = append(errs, validateHTTPSettings(&cfg.Target.Settings)...)
	}

	if len(errs) == 0 {
		return nil
	}
	msg := fmt.Sprintf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	return redteam_errors.NewConfigValidationError(msg)
}

func validateHTTPSettings(s *ConfigSettings) []string {
	var errs []string

	if s.URL == "" {
		errs = append(errs, "target URL is required (set in config file or pass --target-url)")
	} else if err := validateURL(s.URL, "target URL"); err != nil {
		errs = append(errs, err.Error())
	}

	// Skip request body template validation when request_command handles body building.
	if s.RequestCommand != nil {
		if err := s.RequestCommand.Validate("request_command"); err != nil {
			errs = append(errs, err.Error())
		}
	} else {
		if !strings.Contains(s.RequestBodyTemplate, "{{prompt}}") {
			errs = append(errs, "request_body_template must contain the {{prompt}} placeholder")
		}
		replaced := strings.ReplaceAll(s.RequestBodyTemplate, "{{prompt}}", "test")
		if !json.Valid([]byte(replaced)) {
			errs = append(errs, "request_body_template is not valid JSON")
		}
	}

	// Skip JMESPath validation when response_command handles response parsing.
	if s.ResponseCommand != nil {
		if err := s.ResponseCommand.Validate("response_command"); err != nil {
			errs = append(errs, err.Error())
		}
	} else if s.ResponseSelector != "" {
		if _, err := jmespath.Compile(s.ResponseSelector); err != nil {
			errs = append(errs, fmt.Sprintf("response_selector is not a valid JMESPath expression: %v", err))
		}
	}

	if s.URLCommand != nil {
		if err := s.URLCommand.Validate("url_command"); err != nil {
			errs = append(errs, err.Error())
		}
	}

	return errs
}

func validateURL(rawURL, label string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid HTTP(S) URL, got: %q", label, rawURL)
	}
	return nil
}

// NeedsDefaultProfile returns true when no goals or attacks are configured.
func (cfg *Config) NeedsDefaultProfile() bool {
	return len(cfg.Goals) == 0 && len(cfg.Attacks) == 0
}

// resolveAttacks determines which attacks to run based on CLI flags and config.
// Precedence: --goals > --profile > YAML attacks/goals > default "fast" profile.
// Returns the resolved profile name (empty if goals were used directly).
func resolveAttacks(
	config configuration.Configuration,
	client controlserver.Client,
	cfg *Config,
) (string, error) {
	goalsFlag := getGoalsFlag(config)
	profileID := config.GetString(utils.FlagProfile)

	if len(goalsFlag) > 0 && profileID != "" {
		return "", fmt.Errorf("--goals and --profile cannot be used together")
	}

	switch {
	case len(goalsFlag) > 0:
		cfg.Goals = goalsFlag
		cfg.Attacks = nil
		return "", nil
	case profileID != "":
		return applyProfile(context.Background(), client, cfg, profileID)
	case cfg.NeedsDefaultProfile():
		return applyProfile(context.Background(), client, cfg, defaultProfileID)
	default:
		return "", nil
	}
}

func applyProfile(
	ctx context.Context,
	client controlserver.Client,
	cfg *Config,
	profileID string,
) (string, error) {
	profiles, err := client.ListProfiles(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch profiles: %w", err)
	}
	for _, p := range profiles {
		if p.ID == profileID {
			cfg.Attacks = p.Entries
			return p.Name, nil
		}
	}
	return "", fmt.Errorf("profile %q not found on server", profileID)
}

func applyDefaults(cfg *Config) {
	if cfg.Target.Type == "" {
		cfg.Target.Type = defaultTargetType
	}
	// Empty response_selector means "use raw response body as-is" (plain text targets).
	// Skip default template when request_command or target_command handles body building.
	if cfg.Target.Settings.RequestBodyTemplate == "" &&
		cfg.Target.Settings.RequestCommand == nil &&
		cfg.Target.Settings.TargetCommand == nil {
		cfg.Target.Settings.RequestBodyTemplate = defaultRequestBodyTemplate
	}
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
}

func HeadersToMap(hdrs []ConfigHeader) map[string]string {
	headers := make(map[string]string)
	for _, h := range hdrs {
		if h.Name != "" {
			headers[h.Name] = h.Value
		}
	}
	return headers
}

// UniqueGoals returns deduplicated goal names from attacks and goals.
func (cfg *Config) UniqueGoals() []string {
	seen := make(map[string]struct{})
	var goals []string
	for _, a := range cfg.Attacks {
		if _, ok := seen[a.Goal]; !ok {
			seen[a.Goal] = struct{}{}
			goals = append(goals, a.Goal)
		}
	}
	for _, g := range cfg.Goals {
		if _, ok := seen[g]; !ok {
			seen[g] = struct{}{}
			goals = append(goals, g)
		}
	}
	return goals
}

func (cfg *Config) HeadersMap() map[string]string {
	return HeadersToMap(cfg.Target.Settings.Headers)
}

func getToolsFlags(config configuration.Configuration) []string {
	raw := config.GetString(utils.FlagTools)
	if raw == "" {
		return nil
	}
	var tools []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tools = append(tools, t)
		}
	}
	return tools
}

func parseHeaderFlags(config configuration.Configuration) []ConfigHeader {
	raw := config.Get(utils.FlagHeader)
	vals, ok := raw.([]string)
	if !ok || len(vals) == 0 {
		return nil
	}
	headers := make([]ConfigHeader, 0, len(vals))
	for _, h := range vals {
		name, value, found := strings.Cut(h, ":")
		if !found {
			continue
		}
		headers = append(headers, ConfigHeader{
			Name:  strings.TrimSpace(name),
			Value: strings.TrimSpace(value),
		})
	}
	return headers
}

func getInvalidConfigMessage() string {
	return `
	Configuration file is invalid. Please refer to the following example:

	target:
		name: <required, name your target>
		type: <optional, default: http>
		context:
			purpose: '<optional, intended purpose of the target>'
			ground_truth:
				system_prompt: '<optional, ground truth system prompt>'
				tools: '<optional, list of tool names>'
		settings:
			url: '<required, e.g., https://vulnerable-app.com/chat/completions>'
			headers:
				- name: '<optional, e.g. Authorization>'
				  value: '<optional, e.g. Bearer TOKEN>'
			response_selector: '<optional, JMESPath expression; omit for plain text>'
			request_body_template: '<optional, default: {"message": "{{prompt}}"}'
	goals:
		- '<optional, default: system_prompt_extraction>'
	attacks:
		- goal: '<optional, goal name>'
		  strategy: '<optional, strategy name>'

	For more configuration options, refer to the documentation.

	`
}
