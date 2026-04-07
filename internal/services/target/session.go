package target

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// Session mode constants (mirrored from redteam config to avoid circular imports).
const (
	sessionModeNone   = "none"
	sessionModeClient = "client"
)

// SessionStoreConfig is the service-layer configuration for SessionStore.
// It is converted from the YAML-level redteam.SessionConfig by the scan workflow.
type SessionStoreConfig struct {
	Mode string // "none" or "client"
}

// SessionStore manages per-chat session state so that multi-turn conversations
// share a stable session ID when talking to the target.
type SessionStore struct {
	mu       sync.RWMutex
	config   SessionStoreConfig
	sessions map[string]string // chatID -> sessionID
}

// NewSessionStore creates a store that implements the strategy described by cfg.
func NewSessionStore(cfg SessionStoreConfig) *SessionStore {
	return &SessionStore{
		config:   cfg,
		sessions: make(map[string]string),
	}
}

// GetOrCreate returns the session ID for the given chat, creating one if
// this is the first time the chat is seen.
func (s *SessionStore) GetOrCreate(chatID string) (string, error) {
	if s == nil {
		return "", nil
	}

	switch s.config.Mode {
	case sessionModeNone, "":
		return "", nil

	case sessionModeClient:
		return s.getOrGenerate(chatID), nil

	default:
		return "", fmt.Errorf("unknown session mode: %q", s.config.Mode)
	}
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
