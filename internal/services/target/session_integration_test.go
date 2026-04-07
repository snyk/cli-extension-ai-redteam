package target_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
)

const (
	headerContentType   = "Content-Type"
	sessionBodyTemplate = `{"session_id": "{{sessionId}}", "message": "{{prompt}}"}`
	contentTypeJSON     = "application/json"
)

// ---------------------------------------------------------------------------
// sessionTracker — helper that mimics a stateful target server with sessions
// ---------------------------------------------------------------------------

type sessionTracker struct {
	mu       sync.Mutex
	sessions map[string][]string // sessionID -> list of prompts received
}

func newSessionTracker() *sessionTracker {
	return &sessionTracker{sessions: make(map[string][]string)}
}

func (st *sessionTracker) handler(sessionSource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		var sessionID string
		switch sessionSource {
		case "header":
			sessionID = r.Header.Get(headerSessionID)
		case "body":
			sessionID, _ = req["session_id"].(string)
		}

		prompt, _ := req["message"].(string)

		st.mu.Lock()
		st.sessions[sessionID] = append(st.sessions[sessionID], prompt)
		st.mu.Unlock()

		resp := map[string]string{
			"response":   fmt.Sprintf("echo: %s", prompt),
			"session_id": sessionID,
		}
		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(resp)
	}
}

// sendWithSession resolves a session ID from the store and sends a prompt,
// mirroring what the scan loop does in redteam.go.
func sendWithSession(
	t *testing.T,
	client target.Client,
	store *target.SessionStore,
	chatID, prompt string,
) {
	t.Helper()
	sid, err := store.GetOrCreate(chatID)
	require.NoError(t, err)
	ctx := target.WithSessionID(t.Context(), sid)
	_, err = client.SendPrompt(ctx, prompt)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Client mode — session ID in request body
// ---------------------------------------------------------------------------

func TestIntegration_ClientMode_SessionIDInBody(t *testing.T) {
	tracker := newSessionTracker()
	server := httptest.NewServer(tracker.handler("body"))
	defer server.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"})
	client := target.NewHTTPClient(nil, server.URL, nil, sessionBodyTemplate, defaultSelector)

	// Simulate two chats with interleaved prompts (like the real scan loop).
	chats := []struct {
		chatID string
		prompt string
	}{
		{"chat-1", "turn 1 from chat 1"},
		{"chat-2", "turn 1 from chat 2"},
		{"chat-1", "turn 2 from chat 1"},
		{"chat-2", "turn 2 from chat 2"},
		{"chat-1", "turn 3 from chat 1"},
	}

	for _, c := range chats {
		sendWithSession(t, client, store, c.chatID, c.prompt)
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	assert.Len(t, tracker.sessions, 2, "should have exactly 2 sessions (one per chatID)")

	sessIDs := make([]string, 0, len(tracker.sessions))
	for sid, prompts := range tracker.sessions {
		sessIDs = append(sessIDs, sid)
		assert.NotEmpty(t, sid, "session ID should not be empty")
		assert.Len(t, sid, 36, "session ID should be a UUID")
		if len(prompts) == 3 {
			assert.Equal(t, "turn 1 from chat 1", prompts[0])
			assert.Equal(t, "turn 2 from chat 1", prompts[1])
			assert.Equal(t, "turn 3 from chat 1", prompts[2])
		} else {
			assert.Len(t, prompts, 2)
			assert.Equal(t, "turn 1 from chat 2", prompts[0])
			assert.Equal(t, "turn 2 from chat 2", prompts[1])
		}
	}
	assert.NotEqual(t, sessIDs[0], sessIDs[1], "different chats must have different session IDs")
}

// ---------------------------------------------------------------------------
// Client mode — session ID in request header
// ---------------------------------------------------------------------------

func TestIntegration_ClientMode_SessionIDInHeader(t *testing.T) {
	tracker := newSessionTracker()
	server := httptest.NewServer(tracker.handler("header"))
	defer server.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"})
	client := target.NewHTTPClient(
		nil, server.URL,
		map[string]string{headerSessionID: "{{sessionId}}"},
		`{"message": "{{prompt}}"}`,
		defaultSelector,
	)

	sendWithSession(t, client, store, "c1", "hello from c1")
	sendWithSession(t, client, store, "c2", "hello from c2")
	sendWithSession(t, client, store, "c1", "followup c1")

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	assert.Len(t, tracker.sessions, 2)
	for sid, prompts := range tracker.sessions {
		assert.NotEmpty(t, sid)
		if len(prompts) == 2 {
			assert.Equal(t, []string{"hello from c1", "followup c1"}, prompts)
		} else {
			assert.Equal(t, []string{"hello from c2"}, prompts)
		}
	}
}

// ---------------------------------------------------------------------------
// No session — verify backward compatibility
// ---------------------------------------------------------------------------

func TestIntegration_NoSession_BackwardCompatible(t *testing.T) {
	var receivedPrompts []string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)

		mu.Lock()
		msg, _ := req["message"].(string)
		receivedPrompts = append(receivedPrompts, msg)
		mu.Unlock()

		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(map[string]string{"response": "ok"})
	}))
	defer server.Close()

	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, defaultSelector)

	_, err := client.SendPrompt(t.Context(), "no session")
	require.NoError(t, err)

	ctx := target.WithSessionID(t.Context(), "explicit-session")
	_, err = client.SendPrompt(ctx, "with session")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"no session", "with session"}, receivedPrompts)
}

// ---------------------------------------------------------------------------
// Multi-session interleaving — stress test
// ---------------------------------------------------------------------------

func TestIntegration_ClientMode_ManyChatsStayIsolated(t *testing.T) {
	var mu sync.Mutex
	received := make(map[string][]string) // sessionID -> prompts

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		sid, _ := req["session_id"].(string)
		prompt, _ := req["message"].(string)

		mu.Lock()
		received[sid] = append(received[sid], prompt)
		mu.Unlock()

		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(map[string]string{"response": "ok"})
	}))
	defer server.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"})
	client := target.NewHTTPClient(nil, server.URL, nil, sessionBodyTemplate, defaultSelector)

	numChats := 10
	turnsPerChat := 5
	for turn := range turnsPerChat {
		for chatIdx := range numChats {
			chatID := fmt.Sprintf("chat-%d", chatIdx)
			prompt := fmt.Sprintf("chat-%d-turn-%d", chatIdx, turn)
			sendWithSession(t, client, store, chatID, prompt)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	assert.Len(t, received, numChats, "each chat should have its own session ID")
	for _, prompts := range received {
		assert.Len(t, prompts, turnsPerChat, "each session should have received all turns")
	}
}
