package session

import (
	"time"

	"github.com/playwright-community/playwright-go"
)

// BrowserMode represents the mode of browser connection
type BrowserMode string

const (
	ModeLocal    BrowserMode = "local"
	ModeRemote   BrowserMode = "remote"
	ModeAttached BrowserMode = "attached"
)

// Session represents a browser session with multiple pages/tabs
type Session struct {
	Name        string                     // Session name
	Context     playwright.BrowserContext // Playwright BrowserContext
	Pages       []playwright.Page          // Multiple tab pages
	CurrentPage int                        // Current active page index
	Mode        BrowserMode                // Connection mode
	ProfileDir  string                     // Profile directory for persistent mode
	CreatedAt   time.Time                  // Creation timestamp
	BrowserType string                     // Browser type: chromium, firefox, webkit, msedge
	Headless    bool                       // Headless mode
}

// SessionOptions contains options for creating a new session
type SessionOptions struct {
	Browser           string // Browser type: chrome, firefox, webkit, msedge (maps to chromium, firefox, webkit, msedge)
	Headless          bool
	Persistent        bool                   // Use persistent profile
	ProfileDir        string                 // Custom profile directory
	RemoteURL         string                 // Remote browserless URL
	CDPURL            string                 // CDP endpoint URL
	AttachExt         bool                   // Attach via extension
	NoViewport        bool                   // Disable default viewport
	ViewportSize      *ViewportSize          // Custom viewport
	IgnoreHTTPSErrors bool                   // Ignore HTTPS errors
	RecordVideoDir    string                 // Directory for video recording
	RecordVideoSize   *ViewportSize          // Video size
	ExtraPrefs        map[string]interface{} // Extra browser preferences
}

// ViewportSize represents browser viewport dimensions
type ViewportSize struct {
	Width  int
	Height int
}

// NewSession creates a new session with default values
func NewSession(name string) *Session {
	return &Session{
		Name:        name,
		Pages:       make([]playwright.Page, 0),
		CurrentPage: 0,
		CreatedAt:   time.Now(),
	}
}

// NewSessionOptions creates default session options
func NewSessionOptions() *SessionOptions {
	return &SessionOptions{
		Headless:   true,
		Persistent: false,
		Browser:    "chromium",
	}
}
