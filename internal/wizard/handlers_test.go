package wizard_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/go-application-framework/pkg/ui"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	controlservermock "github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver/mock"
	"github.com/snyk/cli-extension-ai-redteam/internal/wizard"
)

type mockUI struct {
	outputs []string
}

func (m *mockUI) Output(output string) error {
	m.outputs = append(m.outputs, output)
	return nil
}

func (m *mockUI) OutputError(_ error, _ ...ui.Opts) error { return nil }

//nolint:ireturn // mock must satisfy the interface
func (m *mockUI) NewProgressBar() ui.ProgressBar { return nil }
func (m *mockUI) Input(_ string) (string, error) { return "", nil }

func (m *mockUI) SelectOptions(_ string, _ []string) (idx int, val string, err error) {
	return 0, "", nil
}

// --- handleGetInitialConfig ---

func TestHandleGetInitialConfig_NilConfig(t *testing.T) {
	handler := wizard.HandleGetInitialConfig(nil, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp wizard.InitialConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Nil(t, resp.Config)
	assert.Empty(t, resp.ConfigPath)
}

func TestHandleGetInitialConfig_WithConfig(t *testing.T) {
	cfg := &redteam.Config{
		Target: redteam.ConfigTarget{Name: "test-target"},
		Goals:  []string{"goal1"},
	}
	handler := wizard.HandleGetInitialConfig(cfg, "/path/to/config.yaml")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp wizard.InitialConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "/path/to/config.yaml", resp.ConfigPath)
	require.NotNil(t, resp.Config)
	assert.Equal(t, "test-target", resp.Config.Target.Name)
}

// --- handlePing ---

func TestHandlePing_Success(t *testing.T) {
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"response": "pong"})
	}))
	defer targetSrv.Close()

	pingReq := wizard.PingRequest{
		URL:                 targetSrv.URL,
		RequestBodyTemplate: `{"message":"{{prompt}}"}`,
		ResponseSelector:    "response",
	}
	body, _ := json.Marshal(pingReq)

	handler := wizard.HandlePing()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, true, resp["success"])
	assert.Equal(t, "pong", resp["response"])
}

func TestHandlePing_HeaderConversion(t *testing.T) {
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "val", r.Header.Get("X-Test"))
		// Empty-name header should be skipped, so no extra header.
		json.NewEncoder(w).Encode(map[string]string{"response": "ok"})
	}))
	defer targetSrv.Close()

	pingReq := wizard.PingRequest{
		URL:                 targetSrv.URL,
		RequestBodyTemplate: `{"message":"{{prompt}}"}`,
		ResponseSelector:    "response",
		Headers: []redteam.ConfigHeader{
			{Name: "X-Test", Value: "val"},
			{Name: "", Value: "should-be-skipped"},
		},
	}
	body, _ := json.Marshal(pingReq)

	handler := wizard.HandlePing()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandlePing_InvalidJSON(t *testing.T) {
	handler := wizard.HandlePing()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{bad"))

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- handleListGoals ---

func TestHandleListGoals_Success(t *testing.T) {
	mock := &controlservermock.MockClient{
		Goals: []controlserver.EnumEntry{
			{Value: "b", DisplayOrder: 2},
			{Value: "a", DisplayOrder: 1},
		},
	}

	handler := wizard.HandleListGoals(mock)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var entries []controlserver.EnumEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&entries))
	require.Len(t, entries, 2)
	assert.Equal(t, "a", entries[0].Value)
	assert.Equal(t, "b", entries[1].Value)
}

func TestHandleListGoals_ClientError(t *testing.T) {
	mock := &controlservermock.MockClient{
		GoalsErr: errors.New("connection failed"),
	}

	handler := wizard.HandleListGoals(mock)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

// --- handleListStrategies ---

func TestHandleListStrategies_Success(t *testing.T) {
	mock := &controlservermock.MockClient{
		Strategies: []controlserver.EnumEntry{
			{Value: "z", DisplayOrder: 3},
			{Value: "y", DisplayOrder: 1},
		},
	}

	handler := wizard.HandleListStrategies(mock)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var entries []controlserver.EnumEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&entries))
	require.Len(t, entries, 2)
	assert.Equal(t, "y", entries[0].Value)
	assert.Equal(t, "z", entries[1].Value)
}

func TestHandleListStrategies_ClientError(t *testing.T) {
	mock := &controlservermock.MockClient{
		StrategiesErr: errors.New("timeout"),
	}

	handler := wizard.HandleListStrategies(mock)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

// --- handleDownloadComplete ---

func TestHandleDownloadComplete_Success(t *testing.T) {
	ui := &mockUI{}
	body, _ := json.Marshal(map[string]string{"filename": "my-config.yaml"})

	handler := wizard.HandleDownloadComplete(ui)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])

	require.Len(t, ui.outputs, 1)
	assert.Contains(t, ui.outputs[0], "Configuration downloaded successfully!")
	assert.Contains(t, ui.outputs[0], "snyk redteam --experimental --config my-config.yaml")
}

func TestHandleDownloadComplete_DefaultFilename(t *testing.T) {
	ui := &mockUI{}
	body, _ := json.Marshal(map[string]string{"filename": ""})

	handler := wizard.HandleDownloadComplete(ui)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, ui.outputs, 1)
	assert.Contains(t, ui.outputs[0], "snyk redteam --experimental --config redteam.yaml")
}

func TestHandleDownloadComplete_InvalidJSON(t *testing.T) {
	ui := &mockUI{}

	handler := wizard.HandleDownloadComplete(ui)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{bad"))

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, ui.outputs)
}
