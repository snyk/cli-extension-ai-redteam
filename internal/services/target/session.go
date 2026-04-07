package target

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jmespath/go-jmespath"

	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

// Session mode constants (mirrored from redteam config to avoid circular imports).
const (
	sessionModeNone     = "none"
	sessionModeClient   = "client"
	sessionModeServer   = "server"
	sessionModeEndpoint = "endpoint"
)

// SessionStoreConfig is the service-layer configuration for SessionStore.
// It is converted from the YAML-level redteam.SessionConfig by the scan workflow.
type SessionStoreConfig struct {
	Mode        string // "none", "client", "server", "endpoint"
	ExtractFrom string // "header:<name>", "body:<jmespath>", "cookie:<name>"
	Endpoint    *SessionEndpointConfig
}

// SessionEndpointConfig describes the HTTP endpoint used to create sessions.
type SessionEndpointConfig struct {
	URL              string
	Method           string
	Headers          map[string]string
	RequestBody      string
	ResponseSelector string
}

// SessionStore manages per-chat session state so that multi-turn conversations
// share a stable session ID when talking to the target.
type SessionStore struct {
	mu         sync.RWMutex
	config     SessionStoreConfig
	sessions   map[string]string // chatID -> sessionID
	httpClient *http.Client
}

// NewSessionStore creates a store that implements the strategy described by cfg.
// httpClient is used only for endpoint mode; pass nil otherwise.
func NewSessionStore(cfg SessionStoreConfig, httpClient *http.Client) *SessionStore {
	return &SessionStore{
		config:     cfg,
		sessions:   make(map[string]string),
		httpClient: httpClient,
	}
}

// GetOrCreate returns the session ID for the given chat, creating one if
// this is the first time the chat is seen.
func (s *SessionStore) GetOrCreate(ctx context.Context, chatID string) (string, error) {
	if s == nil {
		return "", nil
	}

	switch s.config.Mode {
	case sessionModeNone, "":
		return "", nil

	case sessionModeClient:
		return s.getOrGenerate(chatID), nil

	case sessionModeServer:
		// For server mode the ID is populated by OnResponse after the first
		// round-trip. Before that we return an empty string so the first
		// request goes out without a session ID.
		s.mu.RLock()
		id := s.sessions[chatID]
		s.mu.RUnlock()
		return id, nil

	case sessionModeEndpoint:
		return s.getOrCallEndpoint(ctx, chatID)

	default:
		return "", fmt.Errorf("unknown session mode: %q", s.config.Mode)
	}
}

// OnResponse is called after each target response so that server-mode stores
// can extract the session ID from the first reply.
func (s *SessionStore) OnResponse(chatID string, headers http.Header, body []byte) {
	if s == nil || s.config.Mode != sessionModeServer {
		return
	}

	s.mu.RLock()
	_, exists := s.sessions[chatID]
	s.mu.RUnlock()
	if exists {
		return // already captured
	}

	id, err := parseExtractFrom(s.config.ExtractFrom, headers, body)
	if err != nil || id == "" {
		return // extraction failed or empty — will retry on next response
	}

	s.mu.Lock()
	if _, exists := s.sessions[chatID]; !exists {
		s.sessions[chatID] = id
	}
	s.mu.Unlock()
}

// getOrGenerate returns a cached UUID or generates a new one for the chat.
func (s *SessionStore) getOrGenerate(chatID string) string {
	s.mu.RLock()
	id, ok := s.sessions[chatID]
	s.mu.RUnlock()
	if ok {
		return id
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Double-check after acquiring write lock.
	if cached, ok := s.sessions[chatID]; ok {
		return cached
	}
	id = uuid.New().String()
	s.sessions[chatID] = id
	return id
}

// getOrCallEndpoint calls the session endpoint once per chat and caches the result.
func (s *SessionStore) getOrCallEndpoint(ctx context.Context, chatID string) (string, error) {
	s.mu.RLock()
	id, ok := s.sessions[chatID]
	s.mu.RUnlock()
	if ok {
		return id, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.sessions[chatID]; ok {
		return cached, nil
	}

	id, err := s.callSessionEndpoint(ctx)
	if err != nil {
		return "", err
	}
	s.sessions[chatID] = id
	return id, nil
}

func (s *SessionStore) callSessionEndpoint(ctx context.Context) (string, error) {
	ep := s.config.Endpoint
	if ep == nil {
		return "", fmt.Errorf("session endpoint config is nil")
	}

	method := ep.Method
	if method == "" {
		method = http.MethodPost
	}

	var bodyReader io.Reader
	if ep.RequestBody != "" {
		bodyReader = strings.NewReader(ep.RequestBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, ep.URL, bodyReader)
	if err != nil {
		return "", fmt.Errorf("session endpoint request: %w", err)
	}
	if ep.RequestBody != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range ep.Headers {
		req.Header.Set(k, v)
	}

	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("session endpoint call failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("session endpoint read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("session endpoint returned status %d: %s", resp.StatusCode, utils.TruncateBody(respBytes))
	}

	if ep.ResponseSelector == "" {
		return strings.TrimSpace(string(respBytes)), nil
	}

	return extractJMESPath(respBytes, ep.ResponseSelector)
}

// parseExtractFrom parses a spec like "header:X-Session-ID", "body:session_id",
// or "cookie:sid" and returns the extracted value.
func parseExtractFrom(spec string, headers http.Header, body []byte) (string, error) {
	prefix, value, found := strings.Cut(spec, ":")
	if !found {
		return "", fmt.Errorf("invalid extract_from spec: %q", spec)
	}

	switch prefix {
	case "header":
		return headers.Get(value), nil

	case "body":
		return extractJMESPath(body, value)

	case "cookie":
		return extractCookie(headers, value), nil

	default:
		return "", fmt.Errorf("unknown extract_from prefix: %q", prefix)
	}
}

// extractJMESPath parses body as JSON and applies a JMESPath expression.
func extractJMESPath(body []byte, expr string) (string, error) {
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("parse JSON for JMESPath extraction: %w", err)
	}

	result, err := jmespath.Search(expr, data)
	if err != nil {
		return "", fmt.Errorf("JMESPath %q: %w", expr, err)
	}
	if result == nil {
		return "", nil
	}

	switch v := result.(type) {
	case string:
		return v, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("marshal JMESPath result: %w", err)
		}
		return string(b), nil
	}
}

// extractCookie looks for a Set-Cookie header with the given name and returns its value.
func extractCookie(headers http.Header, name string) string {
	resp := &http.Response{Header: headers}
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}
