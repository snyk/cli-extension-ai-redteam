package save_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets/save"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets/testutil"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	controlservermock "github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver/mock"
)

func TestSave_Happy(t *testing.T) {
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
`, testutil.TargetName)
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	ictx := testutil.BaseCtx(t)
	ictx.GetConfiguration().Set("config", configPath)

	mock := &controlservermock.MockClient{
		CreatedTarget: &controlserver.TargetResponse{
			ID:        testutil.TargetID,
			Name:      testutil.TargetName,
			Config:    map[string]any{},
			CreatedAt: "2026-04-01T10:00:00Z",
			UpdatedAt: "2026-04-01T10:00:00Z",
		},
	}

	results, err := save.RunSaveWorkflow(ictx, testutil.MockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	output := string(payload)
	assert.Contains(t, output, testutil.TargetName)
	assert.Contains(t, output, "saved")
	assert.Contains(t, output, testutil.TargetID)

	require.NotNil(t, mock.CreateTargetRequest)
	assert.Equal(t, testutil.TargetName, mock.CreateTargetRequest.Name)
	targetMap, _ := mock.CreateTargetRequest.Config["target"].(map[string]any)
	settingsMap, _ := targetMap["settings"].(map[string]any)
	assert.Nil(t, settingsMap["headers"], "headers should be stripped before save")
}

func TestSave_NameFromFlag(t *testing.T) {
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

	ictx := testutil.BaseCtx(t)
	ictx.GetConfiguration().Set("config", configPath)
	ictx.GetConfiguration().Set("target-name", "flag-name")

	mock := &controlservermock.MockClient{
		CreatedTarget: &controlserver.TargetResponse{
			ID:        testutil.TargetID,
			Name:      "flag-name",
			Config:    map[string]any{},
			CreatedAt: "2026-04-01T10:00:00Z",
			UpdatedAt: "2026-04-01T10:00:00Z",
		},
	}

	results, err := save.RunSaveWorkflow(ictx, testutil.MockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.NotNil(t, mock.CreateTargetRequest)
	assert.Equal(t, "flag-name", mock.CreateTargetRequest.Name)
}

func TestSave_MissingName(t *testing.T) {
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

	ictx := testutil.BaseCtx(t)
	ictx.GetConfiguration().Set("config", configPath)

	_, err := save.RunSaveWorkflow(ictx, testutil.MockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target name is required")
}

func TestSave_ConfigFileNotFound(t *testing.T) {
	ictx := testutil.BaseCtx(t)
	ictx.GetConfiguration().Set("config", "/nonexistent/redteam.yaml")

	_, err := save.RunSaveWorkflow(ictx, testutil.MockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}
