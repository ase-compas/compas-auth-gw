package middleware

import (
	"fmt"
	"sync"
	"time"
)

// MemorySessionStore implements SessionStore interface using in-memory storage
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionData
	cleanup  *time.Ticker
	done     chan bool
}

// NewMemorySessionStore creates a new in-memory session store
func NewMemorySessionStore() *MemorySessionStore {
	store := &MemorySessionStore{
		sessions: make(map[string]*SessionData),
		cleanup:  time.NewTicker(5 * time.Minute), // Cleanup every 5 minutes
		done:     make(chan bool),
	}

	// Start cleanup goroutine
	go store.cleanupExpiredSessions()

	return store
}

// Get retrieves session data by session ID
func (s *MemorySessionStore) Get(sessionID string) (*SessionData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	// Check if session is expired
	if session.ExpiresAt.Before(time.Now()) {
		// Remove expired session
		go func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			delete(s.sessions, sessionID)
		}()
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

// Set stores session data with the given session ID
func (s *MemorySessionStore) Set(sessionID string, data *SessionData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[sessionID] = data
	return nil
}

// Delete removes session data by session ID
func (s *MemorySessionStore) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	return nil
}

// Close stops the cleanup goroutine
func (s *MemorySessionStore) Close() {
	s.cleanup.Stop()
	s.done <- true
}

// cleanupExpiredSessions periodically removes expired sessions
func (s *MemorySessionStore) cleanupExpiredSessions() {
	for {
		select {
		case <-s.cleanup.C:
			s.mu.Lock()
			now := time.Now()
			for sessionID, session := range s.sessions {
				if session.ExpiresAt.Before(now) {
					delete(s.sessions, sessionID)
				}
			}
			s.mu.Unlock()
		case <-s.done:
			return
		}
	}
}

// Size returns the number of active sessions
func (s *MemorySessionStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}
