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
	headerSessionID     = "X-Session-ID"
	headerContentType   = "Content-Type"
	sessionBodyTemplate = `{"session_id": "{{sessionId}}", "message": "{{prompt}}"}`
	contentTypeJSON     = "application/json"
)

// ---------------------------------------------------------------------------
// sessionTracker — helper that mimics a stateful target server with sessions
// ---------------------------------------------------------------------------

// sessionTracker records which session IDs were seen per request and what
// conversation history each session accumulated.
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

// ---------------------------------------------------------------------------
// Client mode — session ID in request body
// ---------------------------------------------------------------------------

func TestIntegration_ClientMode_SessionIDInBody(t *testing.T) {
	tracker := newSessionTracker()
	server := httptest.NewServer(tracker.handler("body"))
	defer server.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode: "client",
	}, nil)

	client := target.NewHTTPClient(
		nil, server.URL, nil,
		sessionBodyTemplate,
		defaultSelector,
		target.WithSessionStore(store),
	)

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
		ctx := target.WithChatContext(t.Context(), target.ChatContext{ChatID: c.chatID})
		_, err := client.SendPrompt(ctx, c.prompt)
		require.NoError(t, err)
	}

	// Verify: two distinct session IDs were used.
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	assert.Len(t, tracker.sessions, 2, "should have exactly 2 sessions (one per chatID)")

	// Collect sessions and verify prompt counts.
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

	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"}, nil)

	client := target.NewHTTPClient(
		nil, server.URL,
		map[string]string{headerSessionID: "{{sessionId}}"},
		`{"message": "{{prompt}}"}`,
		defaultSelector,
		target.WithSessionStore(store),
	)

	ctx1 := target.WithChatContext(t.Context(), target.ChatContext{ChatID: "c1"})
	ctx2 := target.WithChatContext(t.Context(), target.ChatContext{ChatID: "c2"})

	_, err := client.SendPrompt(ctx1, "hello from c1")
	require.NoError(t, err)
	_, err = client.SendPrompt(ctx2, "hello from c2")
	require.NoError(t, err)
	_, err = client.SendPrompt(ctx1, "followup c1")
	require.NoError(t, err)

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
// Server mode — extract session ID from response header
// ---------------------------------------------------------------------------

func TestIntegration_ServerMode_ExtractFromHeader(t *testing.T) {
	// Server assigns a session ID on first request and returns it via header.
	var mu sync.Mutex
	serverSessions := make(map[string]string) // prompt → assigned session ID
	sessionCounter := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)

		// Check if client sent a session ID (on follow-up requests).
		clientSID := r.Header.Get(headerSessionID)
		prompt, _ := req["message"].(string)

		mu.Lock()
		if clientSID == "" {
			// First request — assign new session.
			sessionCounter++
			clientSID = fmt.Sprintf("srv-session-%d", sessionCounter)
		}
		serverSessions[prompt] = clientSID
		mu.Unlock()

		w.Header().Set(headerSessionID, clientSID)
		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(map[string]string{"response": "ok"})
	}))
	defer server.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode:        "server",
		ExtractFrom: "header:X-Session-ID",
	}, nil)

	client := target.NewHTTPClient(
		nil, server.URL,
		map[string]string{headerSessionID: "{{sessionId}}"},
		`{"message": "{{prompt}}"}`,
		defaultSelector,
		target.WithSessionStore(store),
	)

	// Chat A: 3 turns.
	for i := 1; i <= 3; i++ {
		ctx := target.WithChatContext(t.Context(), target.ChatContext{ChatID: "chatA"})
		_, err := client.SendPrompt(ctx, fmt.Sprintf("A-turn-%d", i))
		require.NoError(t, err)
	}

	// Chat B: 2 turns.
	for i := 1; i <= 2; i++ {
		ctx := target.WithChatContext(t.Context(), target.ChatContext{ChatID: "chatB"})
		_, err := client.SendPrompt(ctx, fmt.Sprintf("B-turn-%d", i))
		require.NoError(t, err)
	}

	mu.Lock()
	defer mu.Unlock()

	// A-turn-1 had no session ID (first request), server assigned "srv-session-1".
	assert.Equal(t, "srv-session-1", serverSessions["A-turn-1"])
	// A-turn-2 and A-turn-3 should reuse the extracted session ID.
	assert.Equal(t, "srv-session-1", serverSessions["A-turn-2"])
	assert.Equal(t, "srv-session-1", serverSessions["A-turn-3"])
	// B-turn-1 is a new chat — server assigns "srv-session-2".
	assert.Equal(t, "srv-session-2", serverSessions["B-turn-1"])
	assert.Equal(t, "srv-session-2", serverSessions["B-turn-2"])
}

// ---------------------------------------------------------------------------
// Server mode — extract session ID from response body
// ---------------------------------------------------------------------------

func TestIntegration_ServerMode_ExtractFromBody(t *testing.T) {
	sessionCounter := 0
	var mu sync.Mutex
	serverSessions := make(map[string]string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		prompt, _ := req["message"].(string)
		clientSID, _ := req["session_id"].(string)

		mu.Lock()
		if clientSID == "" {
			sessionCounter++
			clientSID = fmt.Sprintf("body-sess-%d", sessionCounter)
		}
		serverSessions[prompt] = clientSID
		mu.Unlock()

		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(map[string]any{
			"response":   "ok",
			"session_id": clientSID,
		})
	}))
	defer server.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode:        "server",
		ExtractFrom: "body:session_id",
	}, nil)

	client := target.NewHTTPClient(
		nil, server.URL, nil,
		sessionBodyTemplate,
		defaultSelector,
		target.WithSessionStore(store),
	)

	for i := 1; i <= 3; i++ {
		ctx := target.WithChatContext(t.Context(), target.ChatContext{ChatID: "chat-X"})
		_, err := client.SendPrompt(ctx, fmt.Sprintf("turn-%d", i))
		require.NoError(t, err)
	}

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, "body-sess-1", serverSessions["turn-1"])
	assert.Equal(t, "body-sess-1", serverSessions["turn-2"])
	assert.Equal(t, "body-sess-1", serverSessions["turn-3"])
}

// ---------------------------------------------------------------------------
// Server mode — extract session ID from Set-Cookie
// ---------------------------------------------------------------------------

func TestIntegration_ServerMode_ExtractFromCookie(t *testing.T) {
	sessionCounter := 0
	var mu sync.Mutex
	serverSessions := make(map[string]string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		prompt, _ := req["message"].(string)

		// Check if client sends session ID via header (populated from {{sessionId}}).
		clientSID := r.Header.Get(headerSessionID)

		mu.Lock()
		if clientSID == "" {
			sessionCounter++
			clientSID = fmt.Sprintf("cookie-sess-%d", sessionCounter)
		}
		serverSessions[prompt] = clientSID
		mu.Unlock()

		// Return session ID via Set-Cookie.
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: clientSID, Path: "/"})
		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(map[string]string{"response": "ok"})
	}))
	defer server.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode:        "server",
		ExtractFrom: "cookie:sid",
	}, nil)

	client := target.NewHTTPClient(
		nil, server.URL,
		map[string]string{headerSessionID: "{{sessionId}}"},
		`{"message": "{{prompt}}"}`,
		defaultSelector,
		target.WithSessionStore(store),
	)

	for i := 1; i <= 3; i++ {
		ctx := target.WithChatContext(t.Context(), target.ChatContext{ChatID: "chat-cookie"})
		_, err := client.SendPrompt(ctx, fmt.Sprintf("msg-%d", i))
		require.NoError(t, err)
	}

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, "cookie-sess-1", serverSessions["msg-1"])
	assert.Equal(t, "cookie-sess-1", serverSessions["msg-2"])
	assert.Equal(t, "cookie-sess-1", serverSessions["msg-3"])
}

// ---------------------------------------------------------------------------
// Endpoint mode — separate session creation endpoint
// ---------------------------------------------------------------------------

func TestIntegration_EndpointMode(t *testing.T) {
	// Session endpoint that creates sessions.
	epCounter := 0
	var epMu sync.Mutex
	sessionEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "create")

		epMu.Lock()
		epCounter++
		sid := fmt.Sprintf("ep-session-%d", epCounter)
		epMu.Unlock()

		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(map[string]string{"session_id": sid})
	}))
	defer sessionEndpoint.Close()

	// Target that records which session IDs it receives.
	var mu sync.Mutex
	received := make(map[string][]string) // sessionID -> prompts

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		prompt, _ := req["message"].(string)
		sessionID, _ := req["session_id"].(string)

		mu.Lock()
		received[sessionID] = append(received[sessionID], prompt)
		mu.Unlock()

		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(map[string]string{"response": "ok"})
	}))
	defer targetServer.Close()

	store := target.NewSessionStore(target.SessionStoreConfig{
		Mode: "endpoint",
		Endpoint: &target.SessionEndpointConfig{
			URL:              sessionEndpoint.URL,
			Method:           "POST",
			RequestBody:      `{"action": "create"}`,
			ResponseSelector: "session_id",
		},
	}, sessionEndpoint.Client())

	client := target.NewHTTPClient(
		nil, targetServer.URL, nil,
		sessionBodyTemplate,
		defaultSelector,
		target.WithSessionStore(store),
	)

	// Chat A: 2 turns, Chat B: 1 turn.
	for _, c := range []struct {
		chatID, prompt string
	}{
		{"chatA", "A1"},
		{"chatB", "B1"},
		{"chatA", "A2"},
	} {
		ctx := target.WithChatContext(t.Context(), target.ChatContext{ChatID: c.chatID})
		_, err := client.SendPrompt(ctx, c.prompt)
		require.NoError(t, err)
	}

	mu.Lock()
	defer mu.Unlock()
	epMu.Lock()
	defer epMu.Unlock()

	// Endpoint should have been called exactly twice (once per chat).
	assert.Equal(t, 2, epCounter)

	assert.Len(t, received, 2)
	assert.Equal(t, []string{"A1", "A2"}, received["ep-session-1"])
	assert.Equal(t, []string{"B1"}, received["ep-session-2"])
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

	// No session store — original behavior.
	client := target.NewHTTPClient(nil, server.URL, nil, defaultBodyTemplate, defaultSelector)

	// Works with and without ChatContext.
	_, err := client.SendPrompt(t.Context(), "no context")
	require.NoError(t, err)

	ctx := target.WithChatContext(t.Context(), target.ChatContext{ChatID: "chat-1"})
	_, err = client.SendPrompt(ctx, "with context")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"no context", "with context"}, receivedPrompts)
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

	store := target.NewSessionStore(target.SessionStoreConfig{Mode: "client"}, nil)
	client := target.NewHTTPClient(
		nil, server.URL, nil,
		sessionBodyTemplate,
		defaultSelector,
		target.WithSessionStore(store),
	)

	numChats := 10
	turnsPerChat := 5
	for turn := 0; turn < turnsPerChat; turn++ {
		for chatIdx := 0; chatIdx < numChats; chatIdx++ {
			chatID := fmt.Sprintf("chat-%d", chatIdx)
			prompt := fmt.Sprintf("chat-%d-turn-%d", chatIdx, turn)
			ctx := target.WithChatContext(t.Context(), target.ChatContext{ChatID: chatID})
			_, err := client.SendPrompt(ctx, prompt)
			require.NoError(t, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	assert.Len(t, received, numChats, "each chat should have its own session ID")
	for _, prompts := range received {
		assert.Len(t, prompts, turnsPerChat, "each session should have received all turns")
	}
}
