package redteam

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"gopkg.in/yaml.v3"

	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

const (
	defaultControlServerURL     = "http://localhost:8085"
	defaultGoal                 = "system_prompt_extraction"
	defaultResponseSelector     = "response"
	defaultRequestBodyTemplate  = `{"message": "{{prompt}}"}`
)

var defaultStrategies = []string{"directly_asking"}

type RedTeamConfig struct {
	Target           ConfigTarget  `yaml:"target"`
	Options          ConfigOptions `yaml:"options"`
	ControlServerURL string        `yaml:"control_server_url"`
	Goal             string        `yaml:"goal"`
	Strategies       []string      `yaml:"strategies"`
}

type ConfigTarget struct {
	Name     string         `yaml:"name"`
	Type     string         `yaml:"type"`
	Context  ConfigContext  `yaml:"context"`
	Settings ConfigSettings `yaml:"settings"`
}

type ConfigContext struct {
	Purpose string `yaml:"purpose"`
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

type ConfigOptions struct {
	VulnDefinitions ConfigVulnDefinitions `yaml:"vuln_definitions"`
	ScanningAgent   string                `yaml:"scanning_agent,omitempty"`
}

type ConfigVulnDefinitions struct {
	Exclude []string `yaml:"exclude,omitempty"`
}

func LoadAndValidateConfig(logger *zerolog.Logger, config configuration.Configuration) (*RedTeamConfig, []workflow.Data, error) {
	targetURL := config.GetString(utils.FlagTargetURL)
	configPath := config.GetString(utils.FlagConfig)

	var rtConfig RedTeamConfig

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
			return nil, []workflow.Data{newWorkflowData("text/plain", []byte(message))}, nil
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			logger.Debug().Err(err).Msg("error reading config file")
			return nil, []workflow.Data{newWorkflowData("text/plain", []byte(getInvalidConfigMessage()))}, nil
		}

		if err := yaml.Unmarshal(data, &rtConfig); err != nil {
			logger.Debug().Err(err).Msg("error unmarshaling config")
			return nil, []workflow.Data{newWorkflowData("text/plain", []byte(getInvalidConfigMessage()))}, nil
		}
	} else if targetURL == "" {
		message := `No configuration found. Either:
  - Create a redteam.yaml in the current directory
  - Use --config to specify a config file
  - Use --target-url to scan a target directly`
		return nil, []workflow.Data{newWorkflowData("text/plain", []byte(message))}, nil
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

	applyDefaults(&rtConfig)

	if rtConfig.Target.Settings.URL == "" {
		return nil, nil, fmt.Errorf("target URL is required (set in config file or pass --target-url)")
	}

	return &rtConfig, nil, nil
}

func applyDefaults(cfg *RedTeamConfig) {
	if cfg.ControlServerURL == "" {
		cfg.ControlServerURL = defaultControlServerURL
	}
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

func (cfg *RedTeamConfig) HeadersMap() map[string]string {
	headers := make(map[string]string)
	for _, h := range cfg.Target.Settings.Headers {
		headers[h.Name] = h.Value
	}
	return headers
}

func parseHeaderFlags(config configuration.Configuration) []ConfigHeader {
	raw := config.Get(utils.FlagHeaders)
	vals, ok := raw.([]string)
	if !ok || len(vals) == 0 {
		return nil
	}
	var headers []ConfigHeader
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
	Configuration file in invalid. Please refer to the following example:

	target:
		name: <required, name your target>
		type: <required, e.g., api or socket_io>
		settings:
			url: '<required, e.g., https://vulnerable-app.com/chat/completions>'
			headers:
				- name: '<optional, e.g. Authorization>'
				  value: '<optional, e.g. Bearer TOKEN>'
			response_selector: '<optional, default: response>'
			request_body_template: '<optional, default: {"message": "{{prompt}}"}'
	control_server_url: '<optional, default: http://localhost:8085>'
	goal: '<optional, default: system_prompt_extraction>'
	strategies:
		- directly_asking
	
	For more configuration options, refer to the documentation.

	`
}
