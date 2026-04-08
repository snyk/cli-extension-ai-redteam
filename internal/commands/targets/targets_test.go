package targets_test

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/snyk/go-application-framework/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	controlservermock "github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver/mock"
	"github.com/snyk/cli-extension-ai-redteam/mocks/frameworkmock"
)

const (
	testTenantID   = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	testTargetID   = "11111111-2222-3333-4444-555555555555"
	testTargetID2  = "66666666-7777-8888-9999-aaaaaaaaaaaa"
	testTargetName = "my-chatbot"
	cmdSnyk        = "snyk"
	cmdRedteam     = "redteam"
	cmdTargets     = "targets"
	errUsage       = "usage"
	errNotFound    = "not found"
	flagConfig     = "--config"
	cfgKeyConfig   = "config"
)

func mockCSFactory(mock *controlservermock.MockClient) targets.ControlServerFactory {
	return func(_ *zerolog.Logger, _ *http.Client, _, _ string) controlserver.Client {
		return mock
	}
}

func setArgs(args ...string) func() {
	original := os.Args
	os.Args = args
	return func() { os.Args = original }
}

func baseCtx(t *testing.T) *mocks.MockInvocationContext {
	t.Helper()
	ictx := frameworkmock.NewMockInvocationContext(t)
	ictx.GetConfiguration().Set("experimental", true)
	ictx.GetConfiguration().Set("tenant-id", testTenantID)
	return ictx
}

// ---------------------------------------------------------------------------
// Missing experimental flag
// ---------------------------------------------------------------------------

func TestTargetsWorkflow_MissingExperimental(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "list")()
	ictx := frameworkmock.NewMockInvocationContext(t)
	ictx.GetConfiguration().Set("tenant-id", testTenantID)

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

func TestTargetsWorkflow_List_Happy(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "list")()
	ictx := baseCtx(t)

	mock := &controlservermock.MockClient{
		Targets: []controlserver.TargetListItem{
			{ID: testTargetID, Name: testTargetName, CreatedAt: "2026-04-01T10:00:00Z", UpdatedAt: "2026-04-01T10:00:00Z"},
			{ID: testTargetID2, Name: "prod-bot", CreatedAt: "2026-03-30T14:22:00Z", UpdatedAt: "2026-03-30T14:22:00Z"},
		},
	}

	results, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	output := string(payload)
	assert.Contains(t, output, testTargetName)
	assert.Contains(t, output, "prod-bot")
	assert.Contains(t, output, testTargetID)
}

func TestTargetsWorkflow_List_Empty(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "list")()
	ictx := baseCtx(t)

	mock := &controlservermock.MockClient{Targets: []controlserver.TargetListItem{}}

	results, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	assert.Contains(t, string(payload), "No saved targets")
}

func TestTargetsWorkflow_List_ServerError(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "list")()
	ictx := baseCtx(t)

	mock := &controlservermock.MockClient{TargetsErr: fmt.Errorf("server error")}

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server error")
}

// ---------------------------------------------------------------------------
// get
// ---------------------------------------------------------------------------

func TestTargetsWorkflow_Get_MissingArg(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "get")()
	ictx := baseCtx(t)

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errUsage)
}

func TestTargetsWorkflow_Get_NotFound(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "get", "nonexistent")()
	ictx := baseCtx(t)

	mock := &controlservermock.MockClient{
		TargetErr: errors.New(errNotFound),
	}

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errNotFound)
}

func TestTargetsWorkflow_Get_OutputFlag(t *testing.T) {
	outputPath := t.TempDir() + "/downloaded.yaml"

	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "get", testTargetName, "--output", outputPath)()
	ictx := baseCtx(t)
	ictx.GetConfiguration().Set("output", outputPath)

	mock := &controlservermock.MockClient{
		Target: &controlserver.TargetResponse{
			ID:   testTargetID,
			Name: testTargetName,
			Config: map[string]any{
				"target": map[string]any{
					"name": testTargetName,
					"type": "http",
					"settings": map[string]any{
						"url":                   "http://localhost:9000/chat",
						"request_body_template": `{"message": "{{prompt}}"}`,
					},
				},
				"goals": []any{"system_prompt_extraction"},
			},
			CreatedAt: "2026-04-01T10:00:00Z",
			UpdatedAt: "2026-04-01T10:00:00Z",
		},
	}

	results, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	output := string(payload)
	assert.Contains(t, output, testTargetName)
	assert.Contains(t, output, outputPath)
	assert.Contains(t, output, "written to")

	_, statErr := os.Stat(outputPath)
	require.NoError(t, statErr, "output file should exist")
}

func TestTargetsWorkflow_Get_OutputFlag_WritesValidConfig(t *testing.T) {
	outputPath := t.TempDir() + "/config.yaml"

	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "get", testTargetName, "--output", outputPath)()
	ictx := baseCtx(t)
	ictx.GetConfiguration().Set("output", outputPath)

	mock := &controlservermock.MockClient{
		Target: &controlserver.TargetResponse{
			ID:   testTargetID,
			Name: testTargetName,
			Config: map[string]any{
				"target": map[string]any{
					"name": testTargetName,
					"type": "http",
					"settings": map[string]any{
						"url":                   "http://localhost:9000/chat",
						"request_body_template": `{"message": "{{prompt}}"}`,
						"response_selector":     "response",
					},
				},
				"goals": []any{"system_prompt_extraction"},
			},
			CreatedAt: "2026-04-01T10:00:00Z",
			UpdatedAt: "2026-04-01T10:00:00Z",
		},
	}

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.NoError(t, err)

	data, readErr := os.ReadFile(outputPath)
	require.NoError(t, readErr)

	var cfg redteam.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, testTargetName, cfg.Target.Name)
	assert.Equal(t, "http://localhost:9000/chat", cfg.Target.Settings.URL)
	assert.Equal(t, `{"message": "{{prompt}}"}`, cfg.Target.Settings.RequestBodyTemplate)
	assert.Contains(t, cfg.Goals, "system_prompt_extraction")
}

// ---------------------------------------------------------------------------
// save
// ---------------------------------------------------------------------------

func TestTargetsWorkflow_Save_Happy(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/redteam.yaml"
	configContent := fmt.Sprintf(`target:
  name: %s
  type: http
  settings:
    url: http://localhost:8080
    request_body_template: '{"message": "{{prompt}}"}'
    headers:
      - name: Authorization
        value: Bearer secret-token
goals:
  - system_prompt_extraction
`, testTargetName)
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "save", flagConfig, configPath)()
	ictx := baseCtx(t)
	ictx.GetConfiguration().Set(cfgKeyConfig, configPath)

	mock := &controlservermock.MockClient{
		CreatedTarget: &controlserver.TargetResponse{
			ID:        testTargetID,
			Name:      testTargetName,
			Config:    map[string]any{},
			CreatedAt: "2026-04-01T10:00:00Z",
			UpdatedAt: "2026-04-01T10:00:00Z",
		},
	}

	results, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	output := string(payload)
	assert.Contains(t, output, testTargetName)
	assert.Contains(t, output, "saved")
	assert.Contains(t, output, testTargetID)

	require.NotNil(t, mock.CreateTargetRequest)
	assert.Equal(t, testTargetName, mock.CreateTargetRequest.Name)
	// Headers must be stripped before sending.
	targetMap, _ := mock.CreateTargetRequest.Config["target"].(map[string]any)
	settingsMap, _ := targetMap["settings"].(map[string]any)
	assert.Nil(t, settingsMap["headers"], "headers should be stripped before save")
}

func TestTargetsWorkflow_Save_NameFromFlag(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/redteam.yaml"
	configContent := `target:
  type: http
  settings:
    url: http://localhost:8080
    request_body_template: '{"message": "{{prompt}}"}'
goals:
  - system_prompt_extraction
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "save", flagConfig, configPath, "--target-name", "flag-name")()
	ictx := baseCtx(t)
	ictx.GetConfiguration().Set(cfgKeyConfig, configPath)
	ictx.GetConfiguration().Set("target-name", "flag-name")

	mock := &controlservermock.MockClient{
		CreatedTarget: &controlserver.TargetResponse{
			ID:        testTargetID,
			Name:      "flag-name",
			Config:    map[string]any{},
			CreatedAt: "2026-04-01T10:00:00Z",
			UpdatedAt: "2026-04-01T10:00:00Z",
		},
	}

	results, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.NotNil(t, mock.CreateTargetRequest)
	assert.Equal(t, "flag-name", mock.CreateTargetRequest.Name)
}

func TestTargetsWorkflow_Save_MissingName(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/redteam.yaml"
	configContent := `target:
  type: http
  settings:
    url: http://localhost:8080
    request_body_template: '{"message": "{{prompt}}"}'
goals:
  - system_prompt_extraction
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "save", flagConfig, configPath)()
	ictx := baseCtx(t)
	ictx.GetConfiguration().Set(cfgKeyConfig, configPath)

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target name is required")
}

func TestTargetsWorkflow_Save_ConfigFileNotFound(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "save", flagConfig, "/nonexistent/redteam.yaml")()
	ictx := baseCtx(t)

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

// ---------------------------------------------------------------------------
// update
// ---------------------------------------------------------------------------

func TestTargetsWorkflow_Update_Happy(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/redteam.yaml"
	configContent := fmt.Sprintf(`target:
  name: %s
  type: http
  settings:
    url: http://localhost:8080
    request_body_template: '{"message": "{{prompt}}"}'
    headers:
      - name: Authorization
        value: Bearer secret-token
goals:
  - capability_extraction
`, testTargetName)
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "update", testTargetName, flagConfig, configPath)()
	ictx := baseCtx(t)
	ictx.GetConfiguration().Set(cfgKeyConfig, configPath)

	mock := &controlservermock.MockClient{
		UpdatedTarget: &controlserver.TargetResponse{
			ID:        testTargetID,
			Name:      testTargetName,
			Config:    map[string]any{},
			CreatedAt: "2026-04-01T10:00:00Z",
			UpdatedAt: "2026-04-01T12:00:00Z",
		},
	}

	results, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	output := string(payload)
	assert.Contains(t, output, testTargetName)
	assert.Contains(t, output, "updated")

	require.NotNil(t, mock.UpdateTargetRequest)
	assert.Equal(t, testTargetName, mock.UpdateTargetName)
	targetMap, _ := mock.UpdateTargetRequest.Config["target"].(map[string]any)
	settingsMap, _ := targetMap["settings"].(map[string]any)
	assert.Nil(t, settingsMap["headers"], "headers should be stripped before update")
}

func TestTargetsWorkflow_Update_WithRename(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/redteam.yaml"
	configContent := `target:
  type: http
  settings:
    url: http://localhost:8080
    request_body_template: '{"message": "{{prompt}}"}'
goals:
  - system_prompt_extraction
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "update", testTargetName, flagConfig, configPath, "--target-name", "renamed-bot")()
	ictx := baseCtx(t)
	ictx.GetConfiguration().Set(cfgKeyConfig, configPath)
	ictx.GetConfiguration().Set("target-name", "renamed-bot")

	mock := &controlservermock.MockClient{
		UpdatedTarget: &controlserver.TargetResponse{
			ID:        testTargetID,
			Name:      "renamed-bot",
			Config:    map[string]any{},
			CreatedAt: "2026-04-01T10:00:00Z",
			UpdatedAt: "2026-04-01T12:00:00Z",
		},
	}

	results, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.NotNil(t, mock.UpdateTargetRequest)
	assert.Equal(t, testTargetName, mock.UpdateTargetName)
	assert.Equal(t, "renamed-bot", mock.UpdateTargetRequest.Name)
}

func TestTargetsWorkflow_Update_MissingArg(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "update")()
	ictx := baseCtx(t)

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errUsage)
}

func TestTargetsWorkflow_Update_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/redteam.yaml"
	configContent := `target:
  type: http
  settings:
    url: http://localhost:8080
goals:
  - system_prompt_extraction
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "update", "nonexistent", flagConfig, configPath)()
	ictx := baseCtx(t)
	ictx.GetConfiguration().Set(cfgKeyConfig, configPath)

	mock := &controlservermock.MockClient{
		UpdateTgtErr: errors.New(errNotFound),
	}

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errNotFound)
}

// ---------------------------------------------------------------------------
// delete
// ---------------------------------------------------------------------------

func TestTargetsWorkflow_Delete_Happy(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "delete", testTargetName)()
	ictx := baseCtx(t)

	mock := &controlservermock.MockClient{}

	results, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	assert.Contains(t, string(payload), "deleted")
	assert.Equal(t, testTargetName, mock.DeletedTargetName)
}

func TestTargetsWorkflow_Delete_MissingArg(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "delete")()
	ictx := baseCtx(t)

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errUsage)
}

func TestTargetsWorkflow_Delete_NotFound(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "delete", "nonexistent")()
	ictx := baseCtx(t)

	mock := &controlservermock.MockClient{
		DeleteTgtErr: errors.New(errNotFound),
	}

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(mock))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errNotFound)
}

// ---------------------------------------------------------------------------
// unknown sub-action
// ---------------------------------------------------------------------------

func TestTargetsWorkflow_UnknownAction(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets, "unknown")()
	ictx := baseCtx(t)

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errUsage)
}

func TestTargetsWorkflow_NoAction(t *testing.T) {
	defer setArgs(cmdSnyk, cmdRedteam, cmdTargets)()
	ictx := baseCtx(t)

	_, err := targets.RunTargetsWorkflow(ictx, mockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errUsage)
}
