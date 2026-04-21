package browser

import (
	"fmt"

	"github.com/browserless/go-cli-browser/internal/session"
	"github.com/playwright-community/playwright-go"
)

// AttachBrowser implements Browser interface for attaching to existing browsers
type AttachBrowser struct{}

// NewAttachBrowser creates a new AttachBrowser instance
func NewAttachBrowser() *AttachBrowser {
	return &AttachBrowser{}
}

// Launch is not supported for attach browser - returns error
func (b *AttachBrowser) Launch(opts *session.SessionOptions) (*LaunchResult, error) {
	return nil, fmt.Errorf("Launch is not supported for AttachBrowser, use ConnectViaCDP instead")
}

// Connect is not supported for attach browser - returns error
func (b *AttachBrowser) Connect(url string, opts *session.SessionOptions) (playwright.BrowserContext, error) {
	return nil, fmt.Errorf("Connect is not supported for AttachBrowser, use ConnectViaCDP instead")
}

// ConnectViaCDP connects to an existing browser via CDP (Chrome DevTools Protocol)
func (b *AttachBrowser) ConnectViaCDP(url string) (playwright.BrowserContext, error) {
	p, err := getPlaywright()
	if err != nil {
		return nil, err
	}

	browser, err := p.Chromium.ConnectOverCDP(url, playwright.BrowserTypeConnectOverCDPOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect via CDP: %w", err)
	}

	// Get or create default context
	contexts := browser.Contexts()
	if len(contexts) > 0 {
		return contexts[0], nil
	}

	// Create a new context if none exists
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{})
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("failed to create context: %w", err)
	}

	return context, nil
}

// DiscoverChromeEndpoints discovers Chrome/Edge CDP endpoints
func DiscoverChromeEndpoints() []string {
	return []string{
		"ws://localhost:9222",
		"ws://127.0.0.1:9222",
	}
}

// GetCDPURLForChannel returns the CDP URL for a browser channel
func GetCDPURLForChannel(channel string) (string, error) {
	switch channel {
	case "chrome":
		return "ws://localhost:9222", nil
	case "msedge", "edge":
		return "ws://localhost:9222", nil // Edge also uses port 9222
	default:
		// Assume it's a direct URL
		return channel, nil
	}
}
