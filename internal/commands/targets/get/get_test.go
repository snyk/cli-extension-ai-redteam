package get_test

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets/get"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets/testutil"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	controlservermock "github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver/mock"
)

func TestGet_MissingArg(t *testing.T) {
	defer testutil.SetArgs("snyk", "redteam", "targets", "get")()
	ictx := testutil.BaseCtx(t)

	_, err := get.RunGetWorkflow(ictx, testutil.MockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), testutil.ErrUsage)
}

func TestGet_NotFound(t *testing.T) {
	defer testutil.SetArgs("snyk", "redteam", "targets", "get", "nonexistent")()
	ictx := testutil.BaseCtx(t)

	mock := &controlservermock.MockClient{
		TargetErr: errors.New(testutil.ErrNotFound),
	}

	_, err := get.RunGetWorkflow(ictx, testutil.MockCSFactory(mock))
	require.Error(t, err)
	assert.Contains(t, err.Error(), testutil.ErrNotFound)
}

func TestGet_OutputFlag(t *testing.T) {
	outputPath := t.TempDir() + "/downloaded.yaml"

	defer testutil.SetArgs("snyk", "redteam", "targets", "get", testutil.TargetName)()
	ictx := testutil.BaseCtx(t)
	ictx.GetConfiguration().Set("output", outputPath)

	mock := &controlservermock.MockClient{
		Target: &controlserver.TargetResponse{
			ID:   testutil.TargetID,
			Name: testutil.TargetName,
			Config: map[string]any{
				"target": map[string]any{
					"name": testutil.TargetName,
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

	results, err := get.RunGetWorkflow(ictx, testutil.MockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	output := string(payload)
	assert.Contains(t, output, testutil.TargetName)
	assert.Contains(t, output, outputPath)
	assert.Contains(t, output, "written to")

	_, statErr := os.Stat(outputPath)
	require.NoError(t, statErr, "output file should exist")
}

func TestGet_OutputFlag_WritesValidConfig(t *testing.T) {
	outputPath := t.TempDir() + "/config.yaml"

	defer testutil.SetArgs("snyk", "redteam", "targets", "get", testutil.TargetName)()
	ictx := testutil.BaseCtx(t)
	ictx.GetConfiguration().Set("output", outputPath)

	mock := &controlservermock.MockClient{
		Target: &controlserver.TargetResponse{
			ID:   testutil.TargetID,
			Name: testutil.TargetName,
			Config: map[string]any{
				"target": map[string]any{
					"name": testutil.TargetName,
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

	_, err := get.RunGetWorkflow(ictx, testutil.MockCSFactory(mock))
	require.NoError(t, err)

	data, readErr := os.ReadFile(outputPath)
	require.NoError(t, readErr)

	var cfg redteam.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, testutil.TargetName, cfg.Target.Name)
	assert.Equal(t, "http://localhost:9000/chat", cfg.Target.Settings.URL)
	assert.Equal(t, `{"message": "{{prompt}}"}`, cfg.Target.Settings.RequestBodyTemplate)
	assert.Contains(t, cfg.Goals, "system_prompt_extraction")
}
