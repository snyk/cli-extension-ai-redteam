package redteam

import (
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

	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

const (
	defaultResponseSelector    = "response"
	defaultRequestBodyTemplate = `{"message": "{{prompt}}"}`
	defaultTargetType          = "http"
	contentTypePlain           = "text/plain"
)

var defaultGoals = []string{"system_prompt_extraction"}

type Config struct {
	Target  ConfigTarget                `yaml:"target" json:"target"`
	Goals   []string                    `yaml:"goals" json:"goals"`
	Attacks []controlserver.AttackEntry `yaml:"attacks" json:"attacks,omitempty"`
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
	URL                 string         `yaml:"url" json:"url"`
	Headers             []ConfigHeader `yaml:"headers,omitempty" json:"headers,omitempty"`
	ResponseSelector    string         `yaml:"response_selector" json:"response_selector"`
	RequestBodyTemplate string         `yaml:"request_body_template" json:"request_body_template"`
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

	if cfg.Target.Settings.URL == "" {
		errs = append(errs, "target URL is required (set in config file or pass --target-url)")
	} else if err := validateURL(cfg.Target.Settings.URL, "target URL"); err != nil {
		errs = append(errs, err.Error())
	}

	if !strings.Contains(cfg.Target.Settings.RequestBodyTemplate, "{{prompt}}") {
		errs = append(errs, "request_body_template must contain the {{prompt}} placeholder")
	}
	replaced := strings.ReplaceAll(cfg.Target.Settings.RequestBodyTemplate, "{{prompt}}", "test")
	if !json.Valid([]byte(replaced)) {
		errs = append(errs, "request_body_template is not valid JSON")
	}

	if _, err := jmespath.Compile(cfg.Target.Settings.ResponseSelector); err != nil {
		errs = append(errs, fmt.Sprintf("response_selector is not a valid JMESPath expression: %v", err))
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
}

func validateURL(rawURL, label string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid HTTP(S) URL, got: %q", label, rawURL)
	}
	return nil
}

func applyDefaults(cfg *Config) {
	if len(cfg.Goals) == 0 && len(cfg.Attacks) == 0 {
		cfg.Goals = defaultGoals
	}
	if cfg.Target.Type == "" {
		cfg.Target.Type = defaultTargetType
	}
	if cfg.Target.Settings.ResponseSelector == "" {
		cfg.Target.Settings.ResponseSelector = defaultResponseSelector
	}
	if cfg.Target.Settings.RequestBodyTemplate == "" {
		cfg.Target.Settings.RequestBodyTemplate = defaultRequestBodyTemplate
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

func (cfg *Config) HeadersMap() map[string]string {
	return HeadersToMap(cfg.Target.Settings.Headers)
}

func getToolsFlags(config configuration.Configuration) []string {
	raw := config.Get(utils.FlagTools)
	vals, ok := raw.([]string)
	if !ok || len(vals) == 0 {
		return nil
	}
	return vals
}

func parseHeaderFlags(config configuration.Configuration) []ConfigHeader {
	raw := config.Get(utils.FlagHeaders)
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
			response_selector: '<optional, default: response>'
			request_body_template: '<optional, default: {"message": "{{prompt}}"}'
	goals:
		- '<optional, default: system_prompt_extraction>'
	attacks:
		- goal: '<optional, goal name>'
		  strategy: '<optional, strategy name>'

	For more configuration options, refer to the documentation.

	`
}
