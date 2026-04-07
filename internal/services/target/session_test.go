package target_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
)

const testChatID = "chat-1"

// ---------------------------------------------------------------------------
// GetOrCreate — client mode
// ---------------------------------------------------------------------------

func TestSessionStore_ClientMode_GeneratesUUID(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"}, nil)

	id, err := store.GetOrCreate(context.Background(), testChatID)
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Len(t, id, 36) // UUID format
}

func TestSessionStore_ClientMode_StablePerChat(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"}, nil)

	id1, _ := store.GetOrCreate(context.Background(), testChatID)
	id2, _ := store.GetOrCreate(context.Background(), testChatID)
	assert.Equal(t, id1, id2, "same chat_id should return the same session ID")
}

func TestSessionStore_ClientMode_DifferentChatsGetDifferentIDs(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"}, nil)

	id1, _ := store.GetOrCreate(context.Background(), testChatID)
	id2, _ := store.GetOrCreate(context.Background(), "chat-2")
	assert.NotEqual(t, id1, id2, "different chat_ids should get different session IDs")
}

// ---------------------------------------------------------------------------
// GetOrCreate — none mode
// ---------------------------------------------------------------------------

func TestSessionStore_NoneMode_ReturnsEmpty(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "none"}, nil)

	id, err := store.GetOrCreate(context.Background(), testChatID)
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestSessionStore_NilStore_ReturnsEmpty(t *testing.T) {
	var store *target.SessionStore
	id, err := store.GetOrCreate(context.Background(), testChatID)
	require.NoError(t, err)
	assert.Empty(t, id)
}

// ---------------------------------------------------------------------------
// GetOrCreate — server mode
// ---------------------------------------------------------------------------

func TestSessionStore_ServerMode_EmptyBeforeResponse(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode:        "server",
		ExtractFrom: "header:X-Session-ID",
	}, nil)

	id, err := store.GetOrCreate(context.Background(), testChatID)
	require.NoError(t, err)
	assert.Empty(t, id, "no session ID before first response")
}

func TestSessionStore_ServerMode_ExtractFromHeader(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode:        "server",
		ExtractFrom: "header:X-Session-ID",
	}, nil)

	headers := http.Header{}
	headers.Set(headerSessionID, "srv-abc-123")
	store.OnResponse(testChatID, headers, nil)

	id, err := store.GetOrCreate(context.Background(), testChatID)
	require.NoError(t, err)
	assert.Equal(t, "srv-abc-123", id)
}

func TestSessionStore_ServerMode_ExtractFromBody(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode:        "server",
		ExtractFrom: "body:session_id",
	}, nil)

	body := []byte(`{"session_id": "body-sess-456", "response": "hello"}`)
	store.OnResponse(testChatID, http.Header{}, body)

	id, _ := store.GetOrCreate(context.Background(), testChatID)
	assert.Equal(t, "body-sess-456", id)
}

func TestSessionStore_ServerMode_ExtractFromCookie(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode:        "server",
		ExtractFrom: "cookie:sid",
	}, nil)

	headers := http.Header{}
	headers.Add("Set-Cookie", "sid=cookie-value-789; Path=/; HttpOnly")
	store.OnResponse(testChatID, headers, nil)

	id, _ := store.GetOrCreate(context.Background(), testChatID)
	assert.Equal(t, "cookie-value-789", id)
}

func TestSessionStore_ServerMode_OnlyFirstResponseCaptured(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode:        "server",
		ExtractFrom: "header:X-Session-ID",
	}, nil)

	h1 := http.Header{}
	h1.Set(headerSessionID, "first")
	store.OnResponse(testChatID, h1, nil)

	h2 := http.Header{}
	h2.Set(headerSessionID, "second")
	store.OnResponse(testChatID, h2, nil)

	id, _ := store.GetOrCreate(context.Background(), testChatID)
	assert.Equal(t, "first", id, "should keep the first extracted session ID")
}

func TestSessionStore_ServerMode_DifferentChatsIndependent(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode:        "server",
		ExtractFrom: "header:X-Session-ID",
	}, nil)

	h1 := http.Header{}
	h1.Set(headerSessionID, "sess-a")
	store.OnResponse(testChatID, h1, nil)

	h2 := http.Header{}
	h2.Set(headerSessionID, "sess-b")
	store.OnResponse("chat-2", h2, nil)

	id1, _ := store.GetOrCreate(context.Background(), testChatID)
	id2, _ := store.GetOrCreate(context.Background(), "chat-2")
	assert.Equal(t, "sess-a", id1)
	assert.Equal(t, "sess-b", id2)
}

// ---------------------------------------------------------------------------
// GetOrCreate — endpoint mode
// ---------------------------------------------------------------------------

func TestSessionStore_EndpointMode_CallsEndpoint(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"session_id": "ep-sess-%d"}`, callCount)
	}))
	defer srv.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode: "endpoint",
		Endpoint: &target.SessionEndpointConfig{
			URL:              srv.URL,
			Method:           "POST",
			ResponseSelector: "session_id",
		},
	}, srv.Client())

	id, err := store.GetOrCreate(context.Background(), testChatID)
	require.NoError(t, err)
	assert.Equal(t, "ep-sess-1", id)
}

func TestSessionStore_EndpointMode_CachesPerChat(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": "ep-%d"}`, callCount)
	}))
	defer srv.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode: "endpoint",
		Endpoint: &target.SessionEndpointConfig{
			URL:              srv.URL,
			ResponseSelector: "id",
		},
	}, srv.Client())

	id1, _ := store.GetOrCreate(context.Background(), testChatID)
	id2, _ := store.GetOrCreate(context.Background(), testChatID)
	assert.Equal(t, id1, id2, "second call should use cached value")
	assert.Equal(t, 1, callCount, "endpoint should be called only once per chat")
}

func TestSessionStore_EndpointMode_DifferentChatsCallSeparately(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		fmt.Fprintf(w, `{"id": "ep-%d"}`, callCount)
	}))
	defer srv.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode: "endpoint",
		Endpoint: &target.SessionEndpointConfig{
			URL:              srv.URL,
			ResponseSelector: "id",
		},
	}, srv.Client())

	id1, _ := store.GetOrCreate(context.Background(), testChatID)
	id2, _ := store.GetOrCreate(context.Background(), "chat-2")
	assert.NotEqual(t, id1, id2)
	assert.Equal(t, 2, callCount)
}

func TestSessionStore_EndpointMode_PlainTextResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "  plain-session-id  \n")
	}))
	defer srv.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode: "endpoint",
		Endpoint: &target.SessionEndpointConfig{
			URL: srv.URL,
		},
	}, srv.Client())

	id, err := store.GetOrCreate(context.Background(), testChatID)
	require.NoError(t, err)
	assert.Equal(t, "plain-session-id", id)
}

func TestSessionStore_EndpointMode_ErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer srv.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode: "endpoint",
		Endpoint: &target.SessionEndpointConfig{
			URL: srv.URL,
		},
	}, srv.Client())

	_, err := store.GetOrCreate(context.Background(), testChatID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// ---------------------------------------------------------------------------
// parseExtractFrom
// ---------------------------------------------------------------------------

func TestParseExtractFrom_Header(t *testing.T) {
	h := http.Header{}
	h.Set("X-Custom", "value123")
	val, err := target.ParseExtractFrom("header:X-Custom", h, nil)
	require.NoError(t, err)
	assert.Equal(t, "value123", val)
}

func TestParseExtractFrom_HeaderMissing(t *testing.T) {
	val, err := target.ParseExtractFrom("header:X-Missing", http.Header{}, nil)
	require.NoError(t, err)
	assert.Empty(t, val)
}

func TestParseExtractFrom_Body(t *testing.T) {
	body := []byte(`{"data": {"token": "tok-abc"}}`)
	val, err := target.ParseExtractFrom("body:data.token", http.Header{}, body)
	require.NoError(t, err)
	assert.Equal(t, "tok-abc", val)
}

func TestParseExtractFrom_BodyNestedArray(t *testing.T) {
	body := []byte(`{"sessions": [{"id": "first"}]}`)
	val, err := target.ParseExtractFrom("body:sessions[0].id", http.Header{}, body)
	require.NoError(t, err)
	assert.Equal(t, "first", val)
}

func TestParseExtractFrom_BodyInvalidJSON(t *testing.T) {
	_, err := target.ParseExtractFrom("body:key", http.Header{}, []byte("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse JSON")
}

func TestParseExtractFrom_Cookie(t *testing.T) {
	h := http.Header{}
	h.Add("Set-Cookie", "sid=abc123; Path=/")
	h.Add("Set-Cookie", "other=xyz; Path=/")
	val, err := target.ParseExtractFrom("cookie:sid", h, nil)
	require.NoError(t, err)
	assert.Equal(t, "abc123", val)
}

func TestParseExtractFrom_CookieMissing(t *testing.T) {
	val, err := target.ParseExtractFrom("cookie:missing", http.Header{}, nil)
	require.NoError(t, err)
	assert.Empty(t, val)
}

func TestParseExtractFrom_InvalidSpec(t *testing.T) {
	_, err := target.ParseExtractFrom("nocolon", http.Header{}, nil)
	require.Error(t, err)
}

func TestParseExtractFrom_UnknownPrefix(t *testing.T) {
	_, err := target.ParseExtractFrom("query:param", http.Header{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

// ---------------------------------------------------------------------------
// OnResponse — nil store
// ---------------------------------------------------------------------------

func TestSessionStore_OnResponse_NilStore(_ *testing.T) {
	var store *target.SessionStore
	// Should not panic.
	store.OnResponse(testChatID, http.Header{}, nil)
}

// ---------------------------------------------------------------------------
// buildRequestBody — session ID replacement
// ---------------------------------------------------------------------------

func TestBuildRequestBody_WithSessionID(t *testing.T) {
	tmpl := `{"session_id": "{{sessionId}}", "message": "{{prompt}}"}`
	body, err := target.BuildRequestBody(tmpl, "hello world", "sess-123")
	require.NoError(t, err)

	var parsed map[string]string
	require.NoError(t, json.Unmarshal(body, &parsed))
	assert.Equal(t, "sess-123", parsed["session_id"])
	assert.Equal(t, "hello world", parsed["message"])
}

func TestBuildRequestBody_EmptySessionID(t *testing.T) {
	tmpl := `{"session_id": "{{sessionId}}", "message": "{{prompt}}"}`
	body, err := target.BuildRequestBody(tmpl, "hi", "")
	require.NoError(t, err)

	var parsed map[string]string
	require.NoError(t, json.Unmarshal(body, &parsed))
	assert.Equal(t, "", parsed["session_id"])
	assert.Equal(t, "hi", parsed["message"])
}

func TestBuildRequestBody_NoSessionIDPlaceholder(t *testing.T) {
	tmpl := `{"message": "{{prompt}}"}`
	body, err := target.BuildRequestBody(tmpl, "test", "sess-123")
	require.NoError(t, err)

	var parsed map[string]string
	require.NoError(t, json.Unmarshal(body, &parsed))
	assert.Equal(t, "test", parsed["message"])
}

// ---------------------------------------------------------------------------
// resolveHeaders
// ---------------------------------------------------------------------------

func TestResolveHeaders_ReplacesSessionID(t *testing.T) {
	headers := map[string]string{
		"Authorization": "Bearer token",
		headerSessionID: "{{sessionId}}",
	}
	resolved := target.ResolveHeaders(headers, "my-session")
	assert.Equal(t, "Bearer token", resolved["Authorization"])
	assert.Equal(t, "my-session", resolved[headerSessionID])
}

func TestResolveHeaders_EmptySessionID_StillReplaces(t *testing.T) {
	original := map[string]string{
		headerSessionID: "{{sessionId}}",
	}
	resolved := target.ResolveHeaders(original, "")
	assert.Equal(t, "", resolved[headerSessionID])
}

// ---------------------------------------------------------------------------
// ChatContext
// ---------------------------------------------------------------------------

func TestChatContext_RoundTrip(t *testing.T) {
	ctx := context.Background()
	cc := target.ChatContext{ChatID: "chat-42", Seq: 3}
	ctx = target.WithChatContext(ctx, cc)

	got, ok := target.ChatContextFrom(ctx)
	require.True(t, ok)
	assert.Equal(t, "chat-42", got.ChatID)
	assert.Equal(t, 3, got.Seq)
}

func TestChatContext_Missing(t *testing.T) {
	ctx := context.Background()
	got, ok := target.ChatContextFrom(ctx)
	assert.False(t, ok)
	assert.Empty(t, got.ChatID)
}
