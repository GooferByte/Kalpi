// Package session provides a thread-safe in-memory session store.
// Each session maps a UUID session_id to a broker access token so that
// subsequent API calls can look up credentials without re-authenticating.
package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session holds everything the engine needs to call a broker on behalf of a user.
type Session struct {
	ID          string
	Broker      string
	AccessToken string
	APIKey      string // Required by Zerodha ("token api_key:access_token")
	UserID      string
	ExpiresAt   time.Time
}

// Manager is the interface for creating and querying sessions.
// Swap out InMemoryManager for a Redis-backed implementation by satisfying this interface.
type Manager interface {
	// Create stores a new session and returns it with a generated ID.
	Create(broker, accessToken, apiKey, userID string) *Session
	// Get retrieves a live (non-expired) session by ID.
	Get(id string) (*Session, bool)
	// Delete removes a session immediately.
	Delete(id string)
}

type inMemoryManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

// NewInMemoryManager creates a Manager backed by a plain Go map.
// A background goroutine removes expired sessions every 5 minutes.
func NewInMemoryManager(ttlHours int) Manager {
	m := &inMemoryManager{
		sessions: make(map[string]*Session),
		ttl:      time.Duration(ttlHours) * time.Hour,
	}
	go m.cleanup()
	return m
}

func (m *inMemoryManager) Create(broker, accessToken, apiKey, userID string) *Session {
	s := &Session{
		ID:          uuid.New().String(),
		Broker:      broker,
		AccessToken: accessToken,
		APIKey:      apiKey,
		UserID:      userID,
		ExpiresAt:   time.Now().Add(m.ttl),
	}
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s
}

func (m *inMemoryManager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok || time.Now().After(s.ExpiresAt) {
		return nil, false
	}
	return s, true
}

func (m *inMemoryManager) Delete(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

func (m *inMemoryManager) cleanup() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		m.mu.Lock()
		for id, s := range m.sessions {
			if time.Now().After(s.ExpiresAt) {
				delete(m.sessions, id)
			}
		}
		m.mu.Unlock()
	}
}
