package session

import (
	"fmt"
	"sync"

	"github.com/playwright-community/playwright-go"
)

// SessionManager is a thread-safe session storage
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// Global session manager instance
var globalManager *SessionManager

// Init initializes the global session manager
func Init() {
	globalManager = &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// GetManager returns the global session manager
func GetManager() *SessionManager {
	if globalManager == nil {
		Init()
	}
	return globalManager
}

// Get retrieves a session by name
func (m *SessionManager) Get(name string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[name]
	if !ok {
		return nil, fmt.Errorf("session %q not found", name)
	}
	return session, nil
}

// GetOrCreate retrieves a session or creates a new one if it doesn't exist
func (m *SessionManager) GetOrCreate(name string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[name]; ok {
		return session
	}

	session := NewSession(name)
	m.sessions[name] = session
	return session
}

// Set stores a session
func (m *SessionManager) Set(name string, session *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[name] = session
}

// Delete removes a session
func (m *SessionManager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[name]; !ok {
		return fmt.Errorf("session %q not found", name)
	}
	delete(m.sessions, name)
	return nil
}

// List returns all session names
func (m *SessionManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.sessions))
	for name := range m.sessions {
		names = append(names, name)
	}
	return names
}

// ListSessions returns all sessions with details
func (m *SessionManager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// CloseAll closes all browser contexts
func (m *SessionManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, session := range m.sessions {
		if session.Context != nil {
			if err := session.Context.Close(); err != nil {
				lastErr = fmt.Errorf("failed to close session %q: %w", name, err)
			}
		}
	}
	// Clear all sessions but keep the map
	m.sessions = make(map[string]*Session)
	return lastErr
}

// KillAll forcefully kills all browser processes
func (m *SessionManager) KillAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// First try graceful close
	for name, session := range m.sessions {
		if session.Context != nil {
			_ = session.Context.Close()
		}
		delete(m.sessions, name)
	}

	// Note: playwright-go doesn't have a direct kill method
	// The Close() method should terminate processes
	// For force kill, we'd need to implement platform-specific logic
	return nil
}

// CurrentActivePage returns the current active page for a session
func (s *Session) CurrentActivePage() (playwright.Page, error) {
	if len(s.Pages) == 0 {
		return nil, fmt.Errorf("no pages in session")
	}
	if s.CurrentPage < 0 || s.CurrentPage >= len(s.Pages) {
		return nil, fmt.Errorf("invalid page index %d", s.CurrentPage)
	}
	return s.Pages[s.CurrentPage], nil
}

// AddPage adds a new page to the session
func (s *Session) AddPage(page playwright.Page) {
	s.Pages = append(s.Pages, page)
	s.CurrentPage = len(s.Pages) - 1
}

// RemovePage removes a page by index
func (s *Session) RemovePage(index int) error {
	if index < 0 || index >= len(s.Pages) {
		return fmt.Errorf("invalid page index %d", index)
	}
	s.Pages = append(s.Pages[:index], s.Pages[index+1:]...)
	if s.CurrentPage >= len(s.Pages) {
		s.CurrentPage = len(s.Pages) - 1
	}
	if s.CurrentPage < 0 {
		s.CurrentPage = 0
	}
	return nil
}

// SelectPage sets the current page by index
func (s *Session) SelectPage(index int) error {
	if index < 0 || index >= len(s.Pages) {
		return fmt.Errorf("invalid page index %d", index)
	}
	s.CurrentPage = index
	return nil
}
