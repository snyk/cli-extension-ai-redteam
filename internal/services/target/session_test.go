package target_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
)

const (
	testChatID      = "chat-1"
	headerSessionID = "X-Session-ID"
)

// ---------------------------------------------------------------------------
// GetOrCreate — client mode
// ---------------------------------------------------------------------------

func TestSessionStore_ClientMode_GeneratesUUID(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"})

	id, err := store.GetOrCreate(testChatID)
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Len(t, id, 36) // UUID format
}

func TestSessionStore_ClientMode_StablePerChat(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"})

	id1, _ := store.GetOrCreate(testChatID)
	id2, _ := store.GetOrCreate(testChatID)
	assert.Equal(t, id1, id2, "same chat_id should return the same session ID")
}

func TestSessionStore_ClientMode_DifferentChatsGetDifferentIDs(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"})

	id1, _ := store.GetOrCreate(testChatID)
	id2, _ := store.GetOrCreate("chat-2")
	assert.NotEqual(t, id1, id2, "different chat_ids should get different session IDs")
}

// ---------------------------------------------------------------------------
// GetOrCreate — none mode
// ---------------------------------------------------------------------------

func TestSessionStore_NoneMode_ReturnsEmpty(t *testing.T) {
	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "none"})

	id, err := store.GetOrCreate(testChatID)
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestSessionStore_NilStore_ReturnsEmpty(t *testing.T) {
	var store *target.SessionStore
	id, err := store.GetOrCreate(testChatID)
	require.NoError(t, err)
	assert.Empty(t, id)
}

// ---------------------------------------------------------------------------
// WithSessionID / sessionIDFrom
// ---------------------------------------------------------------------------

func TestSessionID_RoundTrip(t *testing.T) {
	ctx := target.WithSessionID(context.Background(), "sess-42")
	assert.Equal(t, "sess-42", target.SessionIDFrom(ctx))
}

func TestSessionID_Missing(t *testing.T) {
	assert.Empty(t, target.SessionIDFrom(context.Background()))
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
