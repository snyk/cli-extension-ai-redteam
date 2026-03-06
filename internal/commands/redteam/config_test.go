package redteam_test

import (
	"io"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
)

func validConfig() *redteam.Config {
	return &redteam.Config{
		Target: redteam.ConfigTarget{
			Name: "test",
			Settings: redteam.ConfigSettings{
				URL:                 "https://example.com/chat",
				ResponseSelector:    "response",
				RequestBodyTemplate: `{"message": "{{prompt}}"}`,
			},
		},
		ControlServerURL: "http://localhost:8085",
		Goal:             "system_prompt_extraction",
		Strategies:       []string{"directly_asking"},
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	err := redteam.ValidateConfig(validConfig())
	require.NoError(t, err)
}

func TestValidateConfig_MissingTargetURL(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.URL = ""
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target URL is required")
}

func TestValidateConfig_InvalidTargetURLScheme(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.URL = "ftp://example.com"
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target URL")
	assert.Contains(t, err.Error(), "valid HTTP(S) URL")
}

func TestValidateConfig_InvalidControlServerURL(t *testing.T) {
	cfg := validConfig()
	cfg.ControlServerURL = "not-a-url"
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "control server URL")
}

func TestValidateConfig_TemplateMissingPlaceholder(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.RequestBodyTemplate = `{"message": "hello"}`
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{{prompt}}")
}

func TestValidateConfig_TemplateInvalidJSON(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.RequestBodyTemplate = `{not json {{prompt}}`
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid JSON")
}

func TestValidateConfig_InvalidJMESPath(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.ResponseSelector = "choices[.invalid"
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response_selector")
	assert.Contains(t, err.Error(), "JMESPath")
}

func TestValidateConfig_ValidJMESPathExpressions(t *testing.T) {
	selectors := []string{
		"response",
		"data.reply",
		"choices[0].message.content",
	}
	for _, sel := range selectors {
		cfg := validConfig()
		cfg.Target.Settings.ResponseSelector = sel
		err := redteam.ValidateConfig(cfg)
		require.NoError(t, err, "selector %q should be valid", sel)
	}
}

func TestValidateConfig_MultipleErrors(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.URL = ""
	cfg.Target.Settings.RequestBodyTemplate = `{broken`
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target URL is required")
	assert.Contains(t, err.Error(), "{{prompt}}")
	assert.Contains(t, err.Error(), "not valid JSON")
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "redteam-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestLoadAndValidateConfig_ControlServerURLFromEnvVar(t *testing.T) {
	configPath := writeConfigFile(t, `
target:
  name: "Test"
  type: api
  settings:
    url: "https://example.com"
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
`)

	t.Setenv("CONTROL_SERVER_URL", "http://custom-server:9090")

	logger := zerolog.New(io.Discard)
	cfg := configuration.New()
	cfg.Set("config", configPath)

	rtConfig, data, err := redteam.LoadAndValidateConfig(&logger, cfg)
	require.NoError(t, err)
	require.Nil(t, data)
	assert.Equal(t, "http://custom-server:9090", rtConfig.ControlServerURL)
}

func TestLoadAndValidateConfig_ControlServerURLConfigFileOverridesEnvVar(t *testing.T) {
	configPath := writeConfigFile(t, `
target:
  name: "Test"
  type: api
  settings:
    url: "https://example.com"
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
control_server_url: "http://from-config:8085"
`)

	t.Setenv("CONTROL_SERVER_URL", "http://from-env:9090")

	logger := zerolog.New(io.Discard)
	cfg := configuration.New()
	cfg.Set("config", configPath)

	rtConfig, data, err := redteam.LoadAndValidateConfig(&logger, cfg)
	require.NoError(t, err)
	require.Nil(t, data)
	assert.Equal(t, "http://from-config:8085", rtConfig.ControlServerURL)
}

func TestLoadAndValidateConfig_ControlServerURLDefaultWhenNoEnvVar(t *testing.T) {
	configPath := writeConfigFile(t, `
target:
  name: "Test"
  type: api
  settings:
    url: "https://example.com"
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
`)

	t.Setenv("CONTROL_SERVER_URL", "")

	logger := zerolog.New(io.Discard)
	cfg := configuration.New()
	cfg.Set("config", configPath)

	rtConfig, data, err := redteam.LoadAndValidateConfig(&logger, cfg)
	require.NoError(t, err)
	require.Nil(t, data)
	assert.Equal(t, "http://localhost:8085", rtConfig.ControlServerURL)
}
