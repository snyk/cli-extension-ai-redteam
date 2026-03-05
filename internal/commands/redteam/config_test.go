package redteam_test

import (
	"testing"

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
