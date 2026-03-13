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
	defaultGoal                = "system_prompt_extraction"
	defaultResponseSelector    = "response"
	defaultRequestBodyTemplate = `{"message": "{{prompt}}"}`
	contentTypePlain           = "text/plain"
)

var defaultStrategies = []string{"directly_asking"}

type Config struct {
	Target     ConfigTarget `yaml:"target"`
	Goal       string       `yaml:"goal"`
	Strategies []string     `yaml:"strategies"`
}

type ConfigTarget struct {
	Name     string         `yaml:"name"`
	Type     string         `yaml:"type"`
	Context  ConfigContext  `yaml:"context"`
	Settings ConfigSettings `yaml:"settings"`
}

type ConfigContext struct {
	Purpose      string   `yaml:"purpose"`
	SystemPrompt string   `yaml:"system_prompt"`
	Tools        []string `yaml:"tools"`
}

type ConfigSettings struct {
	URL                       string         `yaml:"url"`
	Headers                   []ConfigHeader `yaml:"headers,omitempty"`
	ResponseSelector          string         `yaml:"response_selector"`
	RequestBodyTemplate       string         `yaml:"request_body_template"`
	SocketIOPath              string         `yaml:"socketio_path,omitempty"`
	SocketIONamespace         string         `yaml:"socketio_namespace,omitempty"`
	SocketIOSendEventName     string         `yaml:"socketio_send_event_name,omitempty"`
	SocketIOResponseEventName string         `yaml:"socketio_response_event_name,omitempty"`
}

type ConfigHeader struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func LoadAndValidateConfig(logger *zerolog.Logger, config configuration.Configuration) (*Config, []workflow.Data, error) {
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
			return nil, []workflow.Data{newWorkflowData(contentTypePlain, []byte(message))}, nil
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			logger.Debug().Err(err).Msg("error reading config file")
			return nil, []workflow.Data{newWorkflowData(contentTypePlain, []byte(getInvalidConfigMessage()))}, nil
		}

		if err := yaml.Unmarshal(data, &rtConfig); err != nil {
			logger.Debug().Err(err).Msg("error unmarshaling config")
			return nil, []workflow.Data{newWorkflowData(contentTypePlain, []byte(getInvalidConfigMessage()))}, nil
		}
	} else if targetURL == "" {
		message := `No configuration found. Either:
  - Create a redteam.yaml in the current directory
  - Use --config to specify a config file
  - Use --target-url to scan a target directly`
		return nil, []workflow.Data{newWorkflowData(contentTypePlain, []byte(message))}, nil
	}

	if targetURL != "" {
		rtConfig.Target.Settings.URL = targetURL
		if rtConfig.Target.Name == "" {
			rtConfig.Target.Name = targetURL
		}
		if rtConfig.Target.Type == "" {
			rtConfig.Target.Type = "api"
		}
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
		rtConfig.Target.Context.SystemPrompt = v
	}
	if tools := getToolsFlags(config); len(tools) > 0 {
		rtConfig.Target.Context.Tools = tools
	}

	applyDefaults(&rtConfig)

	if err := ValidateConfig(&rtConfig); err != nil {
		return nil, nil, err
	}

	return &rtConfig, nil, nil
}

// ToCreateScanRequest builds the control server StartScan request from config.
// Purpose is sent at top level; ground_truth contains only system_prompt and tools (tools as comma-separated string).
func (cfg *Config) ToCreateScanRequest() *controlserver.CreateScanRequest {
	req := &controlserver.CreateScanRequest{
		Goal:        cfg.Goal,
		Strategies:  cfg.Strategies,
		Purpose:     cfg.Target.Context.Purpose,
		GroundTruth: buildGroundTruthFromContext(&cfg.Target.Context),
	}
	return req
}

func buildGroundTruthFromContext(targetCtx *ConfigContext) *controlserver.GroundTruth {
	if targetCtx.SystemPrompt == "" && len(targetCtx.Tools) == 0 {
		return nil
	}
	gt := &controlserver.GroundTruth{
		SystemPrompt: targetCtx.SystemPrompt,
		Tools:        strings.Join(targetCtx.Tools, ", "),
	}
	return gt
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
	if cfg.Goal == "" {
		cfg.Goal = defaultGoal
	}
	if len(cfg.Strategies) == 0 {
		cfg.Strategies = defaultStrategies
	}
	if cfg.Target.Settings.ResponseSelector == "" {
		cfg.Target.Settings.ResponseSelector = defaultResponseSelector
	}
	if cfg.Target.Settings.RequestBodyTemplate == "" {
		cfg.Target.Settings.RequestBodyTemplate = defaultRequestBodyTemplate
	}
}

func (cfg *Config) HeadersMap() map[string]string {
	headers := make(map[string]string)
	for _, h := range cfg.Target.Settings.Headers {
		headers[h.Name] = h.Value
	}
	return headers
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
		type: <required, e.g., api or socket_io>
		context:
			purpose: '<optional, intended purpose of the target>'
			system_prompt: '<optional, ground truth system prompt>'
			tools: '<optional, list of tool names>'
		settings:
			url: '<required, e.g., https://vulnerable-app.com/chat/completions>'
			headers:
				- name: '<optional, e.g. Authorization>'
				  value: '<optional, e.g. Bearer TOKEN>'
			response_selector: '<optional, default: response>'
			request_body_template: '<optional, default: {"message": "{{prompt}}"}'
	goal: '<optional, default: system_prompt_extraction>'
	strategies:
		- directly_asking
	
	For more configuration options, refer to the documentation.

	`
}
