package browser

import (
	"fmt"

	"github.com/browserless/go-cli-browser/internal/session"
	"github.com/playwright-community/playwright-go"
)

// RemoteBrowser implements Browser interface for remote Browserless connections
type RemoteBrowser struct{}

// NewRemoteBrowser creates a new RemoteBrowser instance
func NewRemoteBrowser() *RemoteBrowser {
	return &RemoteBrowser{}
}

// Launch is not supported for remote browser - returns error
func (b *RemoteBrowser) Launch(opts *session.SessionOptions) (*LaunchResult, error) {
	return nil, fmt.Errorf("Launch is not supported for RemoteBrowser, use Connect instead")
}

// Connect connects to a remote Browserless browser
func (b *RemoteBrowser) Connect(url string, opts *session.SessionOptions) (playwright.BrowserContext, error) {
	browserType, err := getBrowserType(opts.Browser)
	if err != nil {
		return nil, err
	}

	browser, err := browserType.Connect(url, playwright.BrowserTypeConnectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to remote browser: %w", err)
	}

	// Create a new context
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		NoViewport:        playwright.Bool(opts.NoViewport),
		Viewport:          viewportSizeToPV(opts.ViewportSize),
		IgnoreHttpsErrors: playwright.Bool(opts.IgnoreHTTPSErrors),
	})
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("failed to create context: %w", err)
	}

	return context, nil
}

// ConnectViaCDP is not supported for remote browser - returns error
func (b *RemoteBrowser) ConnectViaCDP(url string) (playwright.BrowserContext, error) {
	return nil, fmt.Errorf("ConnectViaCDP is not supported for RemoteBrowser, use AttachBrowser instead")
}
