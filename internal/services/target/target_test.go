package target_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
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

func TestSendPrompt_ServerError_RetriesAndFails(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, defaultSelector)
	_, err := client.SendPrompt(t.Context(), testPrompt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "after 3 attempts")
	assert.Equal(t, int32(3), attempts.Load(), "should retry 3 times on 5xx")
}

func TestSendPrompt_RetriesOnServerError_ThenSucceeds(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte("bad gateway"))
			return
		}
		json.NewEncoder(w).Encode(map[string]string{defaultSelector: "recovered"})
	}))
	defer server.Close()

	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, defaultSelector)
	result, err := client.SendPrompt(t.Context(), testPrompt)
	require.NoError(t, err)
	assert.Equal(t, "recovered", result)
	assert.Equal(t, int32(3), attempts.Load())
}

func TestSendPrompt_RetriesOn4xx(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, defaultSelector)
	_, err := client.SendPrompt(t.Context(), testPrompt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Equal(t, int32(3), attempts.Load(), "should retry on 4xx")
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

// ---------------------------------------------------------------------------
// External command options
// ---------------------------------------------------------------------------

func TestSendPrompt_RequestCommand(t *testing.T) {
	// request_command receives prompt on stdin, produces the HTTP body on stdout.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// The "sh -c" command wraps the prompt in a custom JSON body.
		assert.JSONEq(t, `{"custom":"hello world"}`, string(body))
		json.NewEncoder(w).Encode(map[string]string{defaultSelector: "ok"})
	}))
	defer server.Close()

	cmd := &utils.ExternalCommand{
		Binary: "sh",
		Args:   []string{"-c", `read prompt; printf '{"custom":"%s"}' "$prompt"`},
	}
	client := target.NewHTTPClient(nil, server.URL, nil, "", defaultSelector,
		target.WithRequestCommand(cmd))

	result, err := client.SendPrompt(t.Context(), "hello world")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestSendPrompt_ResponseCommand(t *testing.T) {
	// response_command receives raw body on stdin, produces extracted text on stdout.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"deep":{"nested":"secret value"}}`))
	}))
	defer server.Close()

	// Use a simple sh command to extract the value via grep.
	cmd := &utils.ExternalCommand{
		Binary: "sh",
		Args:   []string{"-c", `cat | sed 's/.*"nested":"//;s/".*//'`},
	}
	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, "",
		target.WithResponseCommand(cmd))

	result, err := client.SendPrompt(t.Context(), testPrompt)
	require.NoError(t, err)
	assert.Equal(t, "secret value", result)
}

func TestSendPrompt_RequestCommandError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach server when request command fails")
	}))
	defer server.Close()

	cmd := &utils.ExternalCommand{
		Binary: "sh",
		Args:   []string{"-c", "exit 1"},
	}
	client := target.NewHTTPClient(nil, server.URL, nil, "", defaultSelector,
		target.WithRequestCommand(cmd))

	_, err := client.SendPrompt(t.Context(), testPrompt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request_command")
}

func TestSendPrompt_ResponseCommandError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{defaultSelector: "ok"})
	}))
	defer server.Close()

	cmd := &utils.ExternalCommand{
		Binary: "sh",
		Args:   []string{"-c", "exit 1"},
	}
	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, "",
		target.WithResponseCommand(cmd))

	_, err := client.SendPrompt(t.Context(), testPrompt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response_command")
}
