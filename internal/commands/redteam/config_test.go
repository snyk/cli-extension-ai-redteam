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
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

const (
	errTargetURLRequired = "target URL is required"

	baseTargetYAML = `
target:
  name: test
  type: http
  settings:
    url: "https://example.com"
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
`
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
		Goals: []string{"system_prompt_extraction"},
	}
}

func testLogger() *zerolog.Logger {
	l := zerolog.New(io.Discard)
	return &l
}

// ---------------------------------------------------------------------------
// ValidateConfig
// ---------------------------------------------------------------------------

func TestValidateConfig_Valid(t *testing.T) {
	err := redteam.ValidateConfig(validConfig())
	require.NoError(t, err)
}

func TestValidateConfig_MissingTargetURL(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.URL = ""
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errTargetURLRequired)
}

func TestValidateConfig_InvalidTargetURLScheme(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.URL = "ftp://example.com"
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target URL")
	assert.Contains(t, err.Error(), "valid HTTP(S) URL")
}

func TestValidateConfig_InvalidTargetURL_NoHost(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.URL = "http://"
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valid HTTP(S) URL")
}

func TestValidateConfig_ValidHTTPSAndHTTP(t *testing.T) {
	for _, u := range []string{"https://example.com", "http://localhost:8080/chat"} {
		cfg := validConfig()
		cfg.Target.Settings.URL = u
		require.NoError(t, redteam.ValidateConfig(cfg), "URL %q should be valid", u)
	}
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

func TestValidateConfig_TemplateBothInvalidJSONAndMissingPlaceholder(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.RequestBodyTemplate = `{broken`
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{{prompt}}")
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
		"output.candidates[0].text",
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
	assert.Contains(t, err.Error(), errTargetURLRequired)
	assert.Contains(t, err.Error(), "{{prompt}}")
	assert.Contains(t, err.Error(), "not valid JSON")
}

func TestValidateConfig_AllFieldsInvalid(t *testing.T) {
	cfg := &redteam.Config{
		Target: redteam.ConfigTarget{
			Settings: redteam.ConfigSettings{
				URL:                 "",
				ResponseSelector:    "[.bad",
				RequestBodyTemplate: `no-json-no-placeholder`,
			},
		},
	}
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errTargetURLRequired)
	assert.Contains(t, err.Error(), "{{prompt}}")
	assert.Contains(t, err.Error(), "not valid JSON")
	assert.Contains(t, err.Error(), "JMESPath")
}

// ---------------------------------------------------------------------------
// HeadersMap
// ---------------------------------------------------------------------------

func TestHeadersMap_Empty(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.Headers = nil
	assert.Empty(t, cfg.HeadersMap())
}

func TestHeadersMap_SingleHeader(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.Headers = []redteam.ConfigHeader{
		{Name: "Authorization", Value: "Bearer tok"},
	}
	m := cfg.HeadersMap()
	assert.Equal(t, map[string]string{"Authorization": "Bearer tok"}, m)
}

func TestHeadersMap_MultipleHeaders(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.Headers = []redteam.ConfigHeader{
		{Name: "Authorization", Value: "Bearer tok"},
		{Name: "X-Custom", Value: "value"},
	}
	m := cfg.HeadersMap()
	assert.Len(t, m, 2)
	assert.Equal(t, "Bearer tok", m["Authorization"])
	assert.Equal(t, "value", m["X-Custom"])
}

func TestHeadersMap_DuplicateKeysLastWins(t *testing.T) {
	cfg := validConfig()
	cfg.Target.Settings.Headers = []redteam.ConfigHeader{
		{Name: "Authorization", Value: "first"},
		{Name: "Authorization", Value: "second"},
	}
	m := cfg.HeadersMap()
	assert.Equal(t, "second", m["Authorization"])
}

// ---------------------------------------------------------------------------
// ToCreateScanRequest
// ---------------------------------------------------------------------------

func TestToCreateScanRequest_WithAttacks(t *testing.T) {
	cfg := validConfig()
	cfg.Attacks = []controlserver.AttackEntry{
		{Goal: "harmful_content", Strategy: "role_play"},
		{Goal: "system_prompt_extraction"},
	}
	req := cfg.ToCreateScanRequest()
	require.Len(t, req.Attacks, 2)
	assert.Equal(t, "harmful_content", req.Attacks[0].Goal)
	assert.Equal(t, "role_play", req.Attacks[0].Strategy)
	assert.Equal(t, "system_prompt_extraction", req.Attacks[1].Goal)
	assert.Empty(t, req.Attacks[1].Strategy)
}

func TestToCreateScanRequest_GoalsConvertedToAttacks(t *testing.T) {
	cfg := validConfig()
	cfg.Goals = []string{"system_prompt_extraction", "harmful_content"}
	cfg.Attacks = nil
	req := cfg.ToCreateScanRequest()
	require.Len(t, req.Attacks, 2)
	assert.Equal(t, "system_prompt_extraction", req.Attacks[0].Goal)
	assert.Empty(t, req.Attacks[0].Strategy)
	assert.Equal(t, "harmful_content", req.Attacks[1].Goal)
	assert.Empty(t, req.Attacks[1].Strategy)
}

func TestToCreateScanRequest_AttacksOverrideGoals(t *testing.T) {
	cfg := validConfig()
	cfg.Goals = []string{"should_be_ignored"}
	cfg.Attacks = []controlserver.AttackEntry{
		{Goal: "used_instead", Strategy: "my_strategy"},
	}
	req := cfg.ToCreateScanRequest()
	require.Len(t, req.Attacks, 1)
	assert.Equal(t, "used_instead", req.Attacks[0].Goal)
}

// ---------------------------------------------------------------------------
// LoadAndValidateConfig — YAML loading
// ---------------------------------------------------------------------------

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "redteam-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestLoadAndValidateConfig_NeedsDefaultProfileWhenNoGoals(t *testing.T) {
	path := writeTestConfig(t, baseTargetYAML)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	rtCfg, data, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Nil(t, data)
	assert.True(t, rtCfg.NeedsDefaultProfile())
}

func TestLoadAndValidateConfig_EmptyResponseSelectorMeansPlainText(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  type: http
  settings:
    url: "https://example.com"
    request_body_template: '{"message": "{{prompt}}"}'
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "", rtCfg.Target.Settings.ResponseSelector)
}

func TestLoadAndValidateConfig_AppliesDefaultRequestBodyTemplate(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  type: http
  settings:
    url: "https://example.com"
    response_selector: "response"
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, `{"message": "{{prompt}}"}`, rtCfg.Target.Settings.RequestBodyTemplate)
}

func TestLoadAndValidateConfig_ExplicitValuesNotOverriddenByDefaults(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  type: http
  settings:
    url: "https://example.com"
    response_selector: "data.reply"
    request_body_template: '{"text": "{{prompt}}", "stream": false}'
goals:
  - "harmful_content"
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"harmful_content"}, rtCfg.Goals)
	assert.Equal(t, "data.reply", rtCfg.Target.Settings.ResponseSelector)
	assert.Contains(t, rtCfg.Target.Settings.RequestBodyTemplate, `"stream": false`)
}

// ---------------------------------------------------------------------------
// LoadAndValidateConfig — flag overrides
// ---------------------------------------------------------------------------

func TestLoadAndValidateConfig_TargetURLFlagOverridesConfig(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  type: http
  settings:
    url: "https://original.com"
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)
	cfg.Set(utils.FlagTargetURL, "https://overridden.com/api")

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "https://overridden.com/api", rtCfg.Target.Settings.URL)
}

func TestLoadAndValidateConfig_TargetURLFlagWithoutConfigFile(t *testing.T) {
	cfg := configuration.New()
	cfg.Set(utils.FlagTargetURL, "https://example.com/chat")

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/chat", rtCfg.Target.Settings.URL)
	assert.Equal(t, "https://example.com/chat", rtCfg.Target.Name)
	assert.Equal(t, "http", rtCfg.Target.Type)
}

func TestLoadAndValidateConfig_TargetURLFlagSetsNameAndTypeIfEmpty(t *testing.T) {
	path := writeTestConfig(t, `
target:
  settings:
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)
	cfg.Set(utils.FlagTargetURL, "https://example.com")

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", rtCfg.Target.Name)
	assert.Equal(t, "http", rtCfg.Target.Type)
}

func TestLoadAndValidateConfig_TargetURLFlagDoesNotOverrideExistingNameAndType(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: "My Chatbot"
  type: http
  settings:
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)
	cfg.Set(utils.FlagTargetURL, "https://example.com")

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "My Chatbot", rtCfg.Target.Name)
	assert.Equal(t, "http", rtCfg.Target.Type)
}

func TestLoadAndValidateConfig_RequestBodyTemplateFlagOverride(t *testing.T) {
	path := writeTestConfig(t, baseTargetYAML)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)
	cfg.Set(utils.FlagRequestBodyTmpl, `{"input": "{{prompt}}", "model": "gpt-4"}`)

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Contains(t, rtCfg.Target.Settings.RequestBodyTemplate, `"model": "gpt-4"`)
}

func TestLoadAndValidateConfig_ResponseSelectorFlagOverride(t *testing.T) {
	path := writeTestConfig(t, baseTargetYAML)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)
	cfg.Set(utils.FlagResponseSelector, "choices[0].message.content")

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "choices[0].message.content", rtCfg.Target.Settings.ResponseSelector)
}

func TestLoadAndValidateConfig_HeaderFlagsAppended(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  type: http
  settings:
    url: "https://example.com"
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
    headers:
      - name: "Existing"
        value: "value"
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)
	cfg.Set(utils.FlagHeader, []string{"Authorization: Bearer tok123", "X-Custom: hello"})

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)

	m := rtCfg.HeadersMap()
	assert.Equal(t, "value", m["Existing"])
	assert.Equal(t, "Bearer tok123", m["Authorization"])
	assert.Equal(t, "hello", m["X-Custom"])
}

func TestLoadAndValidateConfig_HeaderFlagMalformedIgnored(t *testing.T) {
	path := writeTestConfig(t, baseTargetYAML)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)
	cfg.Set(utils.FlagHeader, []string{"no-colon-here", "Good: header"})

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Len(t, rtCfg.Target.Settings.Headers, 1)
	assert.Equal(t, "Good", rtCfg.Target.Settings.Headers[0].Name)
}

func TestLoadAndValidateConfig_HeaderFlagValueWithColons(t *testing.T) {
	path := writeTestConfig(t, baseTargetYAML)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)
	cfg.Set(utils.FlagHeader, []string{"Authorization: Bearer eyJ0:abc:def"})

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Len(t, rtCfg.Target.Settings.Headers, 1)
	assert.Equal(t, "Authorization", rtCfg.Target.Settings.Headers[0].Name)
	assert.Equal(t, "Bearer eyJ0:abc:def", rtCfg.Target.Settings.Headers[0].Value)
}

func TestLoadAndValidateConfig_YAMLHeadersWithColonsInValue(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  type: http
  settings:
    url: "https://example.com"
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
    headers:
      - name: "Authorization"
        value: "Bearer eyJ0:abc:def"
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Len(t, rtCfg.Target.Settings.Headers, 1)
	assert.Equal(t, "Authorization", rtCfg.Target.Settings.Headers[0].Name)
	assert.Equal(t, "Bearer eyJ0:abc:def", rtCfg.Target.Settings.Headers[0].Value)
}

// ---------------------------------------------------------------------------
// LoadAndValidateConfig — error paths
// ---------------------------------------------------------------------------

func TestLoadAndValidateConfig_NoConfigAndNoTargetURL(t *testing.T) {
	cfg := configuration.New()

	_, data, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	require.Len(t, data, 1)
	payload, _ := data[0].GetPayload().([]byte)
	assert.Contains(t, string(payload), "No configuration found")
}

func TestLoadAndValidateConfig_ConfigFileNotFound(t *testing.T) {
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, "/nonexistent/path/redteam.yaml")

	_, data, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	require.Len(t, data, 1)
	payload, _ := data[0].GetPayload().([]byte)
	assert.Contains(t, string(payload), "Configuration file not found")
}

func TestLoadAndValidateConfig_InvalidYAML(t *testing.T) {
	path := writeTestConfig(t, "{{{{ not yaml")
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	_, data, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	require.Len(t, data, 1)
	payload, _ := data[0].GetPayload().([]byte)
	assert.Contains(t, string(payload), "Configuration file is invalid")
}

func TestLoadAndValidateConfig_ValidationError(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  type: http
  settings:
    url: ""
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	_, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errTargetURLRequired)
}

// ---------------------------------------------------------------------------
// External commands (url_command, request_command, response_command)
// ---------------------------------------------------------------------------

func TestLoadAndValidateConfig_URLCommandFromYAML(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  type: http
  settings:
    url_command:
      binary: echo
      args: ["https://resolved.example.com"]
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "https://resolved.example.com", rtCfg.Target.Settings.URL)
}

func TestLoadAndValidateConfig_URLCommandSetsName(t *testing.T) {
	path := writeTestConfig(t, `
target:
  settings:
    url_command:
      binary: echo
      args: ["https://resolved.example.com"]
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "https://resolved.example.com", rtCfg.Target.Name)
}

func TestLoadAndValidateConfig_URLCommandOverridesStaticURL(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  settings:
    url: "https://static.example.com"
    url_command:
      binary: echo
      args: ["https://dynamic.example.com"]
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "https://dynamic.example.com", rtCfg.Target.Settings.URL)
}

func TestLoadAndValidateConfig_URLCommandEmptyOutput(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  settings:
    url_command:
      binary: echo
      args: [""]
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	_, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url_command returned empty output")
}

func TestLoadAndValidateConfig_URLCommandMissingBinary(t *testing.T) {
	cfg := &redteam.Config{
		Target: redteam.ConfigTarget{
			Name: "test",
			Settings: redteam.ConfigSettings{
				URL: "https://example.com",
				URLCommand: &utils.ExternalCommand{
					Binary: "",
				},
				RequestBodyTemplate: `{"message": "{{prompt}}"}`,
			},
		},
	}
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url_command")
	assert.Contains(t, err.Error(), "binary is required")
}

func TestLoadAndValidateConfig_RequestCommandSkipsTemplateValidation(t *testing.T) {
	cfg := &redteam.Config{
		Target: redteam.ConfigTarget{
			Name: "test",
			Settings: redteam.ConfigSettings{
				URL: "https://example.com",
				RequestCommand: &utils.ExternalCommand{
					Binary: "/usr/bin/python",
					Args:   []string{"build_request.py"},
				},
			},
		},
	}
	err := redteam.ValidateConfig(cfg)
	require.NoError(t, err)
}

func TestLoadAndValidateConfig_RequestCommandMissingBinary(t *testing.T) {
	cfg := &redteam.Config{
		Target: redteam.ConfigTarget{
			Name: "test",
			Settings: redteam.ConfigSettings{
				URL: "https://example.com",
				RequestCommand: &utils.ExternalCommand{
					Binary: "",
				},
			},
		},
	}
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request_command")
	assert.Contains(t, err.Error(), "binary is required")
}

func TestLoadAndValidateConfig_ResponseCommandSkipsJMESPathValidation(t *testing.T) {
	cfg := &redteam.Config{
		Target: redteam.ConfigTarget{
			Name: "test",
			Settings: redteam.ConfigSettings{
				URL:              "https://example.com",
				ResponseSelector: "[.invalid",
				ResponseCommand: &utils.ExternalCommand{
					Binary: "/usr/bin/python",
					Args:   []string{"parse_response.py"},
				},
				RequestBodyTemplate: `{"message": "{{prompt}}"}`,
			},
		},
	}
	err := redteam.ValidateConfig(cfg)
	require.NoError(t, err)
}

func TestLoadAndValidateConfig_ResponseCommandMissingBinary(t *testing.T) {
	cfg := &redteam.Config{
		Target: redteam.ConfigTarget{
			Name: "test",
			Settings: redteam.ConfigSettings{
				URL: "https://example.com",
				ResponseCommand: &utils.ExternalCommand{
					Binary: "",
				},
				RequestBodyTemplate: `{"message": "{{prompt}}"}`,
			},
		},
	}
	err := redteam.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response_command")
	assert.Contains(t, err.Error(), "binary is required")
}

func TestLoadAndValidateConfig_RequestCommandFromYAML(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  type: http
  settings:
    url: "https://example.com"
    request_command:
      binary: /usr/bin/python
      args: ["build_request.py"]
    response_selector: "response"
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	require.NotNil(t, rtCfg.Target.Settings.RequestCommand)
	assert.Equal(t, "/usr/bin/python", rtCfg.Target.Settings.RequestCommand.Binary)
	assert.Equal(t, []string{"build_request.py"}, rtCfg.Target.Settings.RequestCommand.Args)
	assert.Empty(t, rtCfg.Target.Settings.RequestBodyTemplate, "default template should not be set when request_command is configured")
}

func TestLoadAndValidateConfig_ResponseCommandFromYAML(t *testing.T) {
	path := writeTestConfig(t, `
target:
  name: test
  type: http
  settings:
    url: "https://example.com"
    response_command:
      binary: /usr/bin/python
      args: ["parse_response.py"]
    request_body_template: '{"message": "{{prompt}}"}'
`)
	cfg := configuration.New()
	cfg.Set(utils.FlagConfig, path)

	rtCfg, _, err := redteam.LoadAndValidateConfig(testLogger(), cfg)
	require.NoError(t, err)
	require.NotNil(t, rtCfg.Target.Settings.ResponseCommand)
	assert.Equal(t, "/usr/bin/python", rtCfg.Target.Settings.ResponseCommand.Binary)
}
