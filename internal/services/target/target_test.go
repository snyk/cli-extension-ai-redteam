package target_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
)

const (
	defaultBodyTemplate = `{"message": "{{prompt}}"}`
	defaultSelector     = "response"
	testPrompt          = "test"
)

func TestSendPrompt_DefaultTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "hello world", req["message"])

		json.NewEncoder(w).Encode(map[string]string{defaultSelector: "hi there"})
	}))
	defer server.Close()

	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, defaultSelector)
	result, err := client.SendPrompt(t.Context(), "hello world")
	require.NoError(t, err)
	assert.Equal(t, "hi there", result)
}

func TestSendPrompt_NestedResponseSelector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"reply": "nested value",
			},
		})
	}))
	defer server.Close()

	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, "data.reply")
	result, err := client.SendPrompt(t.Context(), testPrompt)
	require.NoError(t, err)
	assert.Equal(t, "nested value", result)
}

func TestSendPrompt_CustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom"))
		json.NewEncoder(w).Encode(map[string]string{defaultSelector: "ok"})
	}))
	defer server.Close()

	headers := map[string]string{
		"Authorization": "Bearer token123",
		"X-Custom":      "custom-value",
	}
	client := target.NewHTTPClient(nil, server.URL, headers, defaultBodyTemplate, defaultSelector)
	result, err := client.SendPrompt(t.Context(), testPrompt)
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestSendPrompt_SpecialCharactersInPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "line1\nline2\t\"quoted\"\nend", req["message"])
		json.NewEncoder(w).Encode(map[string]string{defaultSelector: "ok"})
	}))
	defer server.Close()

	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, defaultSelector)
	result, err := client.SendPrompt(t.Context(), "line1\nline2\t\"quoted\"\nend")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestSendPrompt_ComplexTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "gpt-4", req["model"])
		messages, ok := req["messages"].([]any)
		require.True(t, ok)
		msg, ok := messages[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user", msg["role"])
		assert.Equal(t, "say hello", msg["content"])

		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{"message": map[string]any{"content": "Hello!"}},
			},
		})
	}))
	defer server.Close()

	template := `{"model": "gpt-4", "messages": [{"role": "user", "content": "{{prompt}}"}]}`
	client := target.NewHTTPClient(nil, server.URL, nil, template, "choices")
	result, err := client.SendPrompt(t.Context(), "say hello")
	require.NoError(t, err)
	assert.Contains(t, result, "Hello!")
}

func TestSendPrompt_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, defaultSelector)
	_, err := client.SendPrompt(t.Context(), testPrompt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestSendPrompt_ArrayIndexSelector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{"message": map[string]any{"content": "Hello!"}},
			},
		})
	}))
	defer server.Close()

	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, "choices[0].message.content")
	result, err := client.SendPrompt(t.Context(), testPrompt)
	require.NoError(t, err)
	assert.Equal(t, "Hello!", result)
}

func TestSendPrompt_InvalidResponseSelector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"other": "value"})
	}))
	defer server.Close()

	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, defaultSelector)
	_, err := client.SendPrompt(t.Context(), testPrompt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no match found")
}
