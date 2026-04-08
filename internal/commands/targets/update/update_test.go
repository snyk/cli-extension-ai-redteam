package update_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets/testutil"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets/update"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	controlservermock "github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver/mock"
)

func TestUpdate_Happy(t *testing.T) {
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
`, testutil.TargetName)
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	defer testutil.SetArgs("snyk", "redteam", "targets", "update", testutil.TargetName)()
	ictx := testutil.BaseCtx(t)
	ictx.GetConfiguration().Set("config", configPath)

	mock := &controlservermock.MockClient{
		UpdatedTarget: &controlserver.TargetResponse{
			ID:        testutil.TargetID,
			Name:      testutil.TargetName,
			Config:    map[string]any{},
			CreatedAt: "2026-04-01T10:00:00Z",
			UpdatedAt: "2026-04-01T12:00:00Z",
		},
	}

	results, err := update.RunUpdateWorkflow(ictx, testutil.MockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	output := string(payload)
	assert.Contains(t, output, testutil.TargetName)
	assert.Contains(t, output, "updated")

	require.NotNil(t, mock.UpdateTargetRequest)
	assert.Equal(t, testutil.TargetName, mock.UpdateTargetName)
	targetMap, _ := mock.UpdateTargetRequest.Config["target"].(map[string]any)
	settingsMap, _ := targetMap["settings"].(map[string]any)
	assert.Nil(t, settingsMap["headers"], "headers should be stripped before update")
}

func TestUpdate_WithRename(t *testing.T) {
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

	defer testutil.SetArgs("snyk", "redteam", "targets", "update", testutil.TargetName)()
	ictx := testutil.BaseCtx(t)
	ictx.GetConfiguration().Set("config", configPath)
	ictx.GetConfiguration().Set("target-name", "renamed-bot")

	mock := &controlservermock.MockClient{
		UpdatedTarget: &controlserver.TargetResponse{
			ID:        testutil.TargetID,
			Name:      "renamed-bot",
			Config:    map[string]any{},
			CreatedAt: "2026-04-01T10:00:00Z",
			UpdatedAt: "2026-04-01T12:00:00Z",
		},
	}

	results, err := update.RunUpdateWorkflow(ictx, testutil.MockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.NotNil(t, mock.UpdateTargetRequest)
	assert.Equal(t, testutil.TargetName, mock.UpdateTargetName)
	assert.Equal(t, "renamed-bot", mock.UpdateTargetRequest.Name)
}

func TestUpdate_MissingArg(t *testing.T) {
	defer testutil.SetArgs("snyk", "redteam", "targets", "update")()
	ictx := testutil.BaseCtx(t)

	_, err := update.RunUpdateWorkflow(ictx, testutil.MockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), testutil.ErrUsage)
}

func TestUpdate_NotFound(t *testing.T) {
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

	defer testutil.SetArgs("snyk", "redteam", "targets", "update", "nonexistent")()
	ictx := testutil.BaseCtx(t)
	ictx.GetConfiguration().Set("config", configPath)

	mock := &controlservermock.MockClient{
		UpdateTgtErr: errors.New(testutil.ErrNotFound),
	}

	_, err := update.RunUpdateWorkflow(ictx, testutil.MockCSFactory(mock))
	require.Error(t, err)
	assert.Contains(t, err.Error(), testutil.ErrNotFound)
}
