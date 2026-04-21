package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SessionState represents the persisted session information
type SessionState struct {
	Name        string `json:"name"`
	CDPPort     int    `json:"cdp_port"`
	Pid         int    `json:"pid"`
	BrowserType string `json:"browser_type"`
	Headless    bool   `json:"headless"`
}

// PersistenceManager handles session persistence to disk
type PersistenceManager struct {
	mu        sync.RWMutex
	stateDir  string
	sessions  map[string]*SessionState
}

var (
	persistenceMgr *PersistenceManager
	persistenceMu  sync.Once
)

// GetPersistenceManager returns the global persistence manager
func GetPersistenceManager() *PersistenceManager {
	persistenceMu.Do(func() {
		homeDir, _ := os.UserHomeDir()
		stateDir := filepath.Join(homeDir, ".go-cli-browser", "sessions")
		os.MkdirAll(stateDir, 0755)

		persistenceMgr = &PersistenceManager{
			stateDir: stateDir,
			sessions: make(map[string]*SessionState),
		}
		persistenceMgr.loadAll()
	})
	return persistenceMgr
}

// loadAll loads all session states from disk
func (m *PersistenceManager) loadAll() {
	entries, err := os.ReadDir(m.stateDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(m.stateDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var state SessionState
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}

		m.sessions[state.Name] = &state
	}
}

// Save saves a session state to disk
func (m *PersistenceManager) Save(state *SessionState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[state.Name] = state

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %w", err)
	}

	path := filepath.Join(m.stateDir, state.Name+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session state: %w", err)
	}

	return nil
}

// Get retrieves a session state by name
func (m *PersistenceManager) Get(name string) (*SessionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.sessions[name]
	if !ok {
		return nil, fmt.Errorf("session %q not found", name)
	}
	return state, nil
}

// Delete removes a session state from disk
func (m *PersistenceManager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, name)

	path := filepath.Join(m.stateDir, name+".json")
	os.Remove(path)

	return nil
}

// List returns all session names
func (m *PersistenceManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.sessions))
	for name := range m.sessions {
		names = append(names, name)
	}
	return names
}

// IsActive checks if a session is still active (process is running)
func (m *PersistenceManager) IsActive(state *SessionState) bool {
	if state == nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(state.Pid)
	if err != nil {
		return false
	}

	// Try to signal the process - if it succeeds, it's still running
	err = process.Signal(os.Signal(nil))
	return err == nil
}

// GetCDPURL returns the CDP URL for a session
func (m *PersistenceManager) GetCDPURL(name string) (string, error) {
	state, err := m.Get(name)
	if err != nil {
		return "", err
	}

	if !m.IsActive(state) {
		m.Delete(name)
		return "", fmt.Errorf("session %q is no longer active", name)
	}

	return fmt.Sprintf("ws://localhost:%d", state.CDPPort), nil
}
