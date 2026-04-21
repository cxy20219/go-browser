package browser

import (
	"fmt"
	"net"
	"os"

	"github.com/browserless/go-cli-browser/internal/session"
	"github.com/playwright-community/playwright-go"
)

// LocalBrowser implements Browser interface for local browser launch
type LocalBrowser struct{}

// NewLocalBrowser creates a new LocalBrowser instance
func NewLocalBrowser() *LocalBrowser {
	return &LocalBrowser{}
}

// Launch starts a local browser instance
func (b *LocalBrowser) Launch(opts *session.SessionOptions) (*LaunchResult, error) {
	browserType, err := getBrowserType(opts.Browser)
	if err != nil {
		return nil, err
	}

	var browser playwright.Browser
	var context playwright.BrowserContext

	cdpPort, err := findFreePort()
	if err != nil {
		return nil, err
	}

	if opts.Persistent {
		// Use persistent context
		userDataDir := opts.ProfileDir
		if userDataDir == "" {
			userDataDir = opts.Browser + "-profile"
		}

		persistentContext, err := browserType.LaunchPersistentContext(userDataDir, playwright.BrowserTypeLaunchPersistentContextOptions{
			Headless: playwright.Bool(opts.Headless),
			Args: []string{
				"--disable-web-security",
				fmt.Sprintf("--remote-debugging-port=%d", cdpPort),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to launch persistent context: %w", err)
		}
		return &LaunchResult{
			Context: persistentContext,
			Pid:     os.Getpid(),
			CDPPort: cdpPort,
		}, nil
	}

	// Launch regular browser with CDP port
	browser, err = browserType.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(opts.Headless),
		Args: []string{
			fmt.Sprintf("--remote-debugging-port=%d", cdpPort),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	// Create a new context
	context, err = browser.NewContext(playwright.BrowserNewContextOptions{
		NoViewport:        playwright.Bool(opts.NoViewport),
		Viewport:          viewportSizeToPV(opts.ViewportSize),
		IgnoreHttpsErrors: playwright.Bool(opts.IgnoreHTTPSErrors),
	})
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("failed to create context: %w", err)
	}

	return &LaunchResult{
		Browser: browser,
		Context: context,
		Pid:     os.Getpid(),
		CDPPort: cdpPort,
	}, nil
}

func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find free CDP port: %w", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("failed to inspect free CDP port")
	}
	return addr.Port, nil
}

// Connect is not supported for local browser - returns error
func (b *LocalBrowser) Connect(url string, opts *session.SessionOptions) (playwright.BrowserContext, error) {
	return nil, fmt.Errorf("Connect is not supported for LocalBrowser, use RemoteBrowser instead")
}

// ConnectViaCDP connects to an existing browser via CDP
func (b *LocalBrowser) ConnectViaCDP(url string) (playwright.BrowserContext, error) {
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
