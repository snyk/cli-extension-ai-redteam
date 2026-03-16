package target

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_truncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"long string cut with ellipsis", "hello world", 5, "hello..."},
		{"empty string", "", 10, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, truncate(tt.input, tt.maxLen))
		})
	}
}

func Test_prettyJSON(t *testing.T) {
	t.Run("valid JSON indented", func(t *testing.T) {
		raw := []byte(`{"a":"b","c":1}`)
		result := prettyJSON(raw, string(raw))
		assert.Contains(t, result, "  \"a\": \"b\"")
	})

	t.Run("invalid JSON returns truncated fallback", func(t *testing.T) {
		raw := []byte(`not json`)
		result := prettyJSON(raw, "not json")
		assert.Equal(t, "not json", result)
	})

	t.Run("large JSON truncated at 2000", func(t *testing.T) {
		obj := make(map[string]string)
		for i := 0; i < 500; i++ {
			obj[strings.Repeat("k", 5)+string(rune('a'+i%26))] = strings.Repeat("v", 10)
		}
		raw, _ := json.Marshal(obj)
		result := prettyJSON(raw, string(raw))
		assert.LessOrEqual(t, len(result), 2003) // 2000 + "..."
	})
}

func Test_extractJMESPaths(t *testing.T) {
	t.Run("flat object", func(t *testing.T) {
		data := map[string]any{"a": "x", "b": "y"}
		paths := extractJMESPaths(data, "", 3)
		assert.Equal(t, []string{"a", "b"}, paths)
	})

	t.Run("nested object", func(t *testing.T) {
		data := map[string]any{"data": map[string]any{"reply": "ok"}}
		paths := extractJMESPaths(data, "", 3)
		assert.Equal(t, []string{"data.reply"}, paths)
	})

	t.Run("array", func(t *testing.T) {
		data := map[string]any{"items": []any{map[string]any{"id": 1}}}
		paths := extractJMESPaths(data, "", 3)
		assert.Equal(t, []string{"items[0].id"}, paths)
	})

	t.Run("depth limit respected", func(t *testing.T) {
		data := map[string]any{"a": map[string]any{"b": map[string]any{"c": "deep"}}}
		paths := extractJMESPaths(data, "", 1)
		assert.Equal(t, []string{"a"}, paths)
	})

	t.Run("nil input", func(t *testing.T) {
		paths := extractJMESPaths(nil, "", 3)
		assert.Empty(t, paths)
	})

	t.Run("empty map", func(t *testing.T) {
		paths := extractJMESPaths(map[string]any{}, "", 3)
		assert.Empty(t, paths)
	})
}

// --- Ping integration tests ---

func newPingClient(url string, headers map[string]string, bodyTemplate, selector string) *HTTPClient {
	return NewHTTPClient(nil, url, headers, bodyTemplate, selector)
}

func TestPing_Success_WithSelector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"response": "hello"})
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.True(t, result.Success)
	assert.Equal(t, "hello", result.Response)
}

func TestPing_Success_EmptySelector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("plain text response"))
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "")
	result := client.Ping(context.Background())
	assert.True(t, result.Success)
	assert.Equal(t, "plain text response", result.Response)
}

func TestPing_Success_NestedSelector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"reply": "ok"}})
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "data.reply")
	result := client.Ping(context.Background())
	assert.True(t, result.Success)
	assert.Equal(t, "ok", result.Response)
}

func TestPing_InvalidRequestBodyTemplate(t *testing.T) {
	client := newPingClient("http://unused", nil, `{invalid`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Suggestion, "template")
}

func TestPing_HTTP401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.Contains(t, strings.ToLower(result.Suggestion), "authentication")
}

func TestPing_HTTP403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.Contains(t, strings.ToLower(result.Suggestion), "authentication")
}

func TestPing_HTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.Contains(t, strings.ToLower(result.Suggestion), "not found")
}

func TestPing_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("oops"))
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.Contains(t, strings.ToLower(result.Suggestion), "server error")
}

func TestPing_HTTP302_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			w.WriteHeader(http.StatusFound)
			return
		}
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

func TestPing_NonJSONResponse_WithSelector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("this is plain text"))
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.Contains(t, strings.ToLower(result.Error), "non-json")
}

func TestPing_SelectorNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"other": "value"})
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.Contains(t, result.Suggestion, "response")
	assert.Contains(t, result.AvailableKeys, "other")
}

func TestPing_SelectorNoMatch_ShowsNestedKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"reply": "ok"},
		})
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "nonexistent")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.Contains(t, result.AvailableKeys, "data.reply")
}

func TestPing_ConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := srv.URL
	srv.Close()

	client := newPingClient(url, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.Contains(t, strings.ToLower(result.Suggestion), "unreachable")
}

func TestPing_CustomHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom"))
		json.NewEncoder(w).Encode(map[string]string{"response": "ok"})
	}))
	defer srv.Close()

	headers := map[string]string{
		"Authorization": "Bearer token123",
		"X-Custom":      "custom-value",
	}
	client := newPingClient(srv.URL, headers, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.True(t, result.Success)
}

func TestPing_RequestBodyContainsPingMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, PingMessage, req["message"])
		json.NewEncoder(w).Encode(map[string]string{"response": "ok"})
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.True(t, result.Success)
}

func TestPing_RawBodyTruncatedOnError(t *testing.T) {
	longBody := strings.Repeat("x", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(longBody))
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.LessOrEqual(t, len(result.RawBody), 503) // 500 + "..."
}

func TestPing_EmptyBody_WithSelector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newPingClient(srv.URL, nil, `{"message":"{{prompt}}"}`, "response")
	result := client.Ping(context.Background())
	assert.False(t, result.Success)
	assert.Contains(t, strings.ToLower(result.Error), "non-json")
}
