package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/browserless/go-cli-browser/internal/session"
	"github.com/playwright-community/playwright-go"
)

// Browser interface defines methods for browser connections
type Browser interface {
	// Launch starts a local browser
	Launch(opts *session.SessionOptions) (*LaunchResult, error)
	// Connect connects to a remote browser
	Connect(url string, opts *session.SessionOptions) (playwright.BrowserContext, error)
	// ConnectViaCDP connects to an existing browser via CDP
	ConnectViaCDP(url string) (playwright.BrowserContext, error)
}

// LaunchResult contains the result of launching a browser
type LaunchResult struct {
	Browser playwright.Browser
	Context playwright.BrowserContext
	Pid     int
	CDPPort int
}

// PlaywrightInstance holds the global Playwright instance
var (
	pw      *playwright.Playwright
	pwMutex sync.Once
	pwErr   error
)

// getPlaywright returns the global Playwright instance
func getPlaywright() (*playwright.Playwright, error) {
	pwMutex.Do(func() {
		includeUserPlaywrightDeps()
		pw, pwErr = playwright.Run()
	})
	if pwErr != nil {
		return nil, fmt.Errorf("failed to run playwright: %w", pwErr)
	}
	return pw, nil
}

func includeUserPlaywrightDeps() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	depsDir := filepath.Join(home, ".local", "playwright-deps", "root", "usr", "lib", "x86_64-linux-gnu")
	if _, err := os.Stat(depsDir); err != nil {
		return
	}

	current := os.Getenv("LD_LIBRARY_PATH")
	for _, dir := range filepath.SplitList(current) {
		if dir == depsDir {
			return
		}
	}
	if current == "" {
		os.Setenv("LD_LIBRARY_PATH", depsDir)
		return
	}
	os.Setenv("LD_LIBRARY_PATH", depsDir+string(os.PathListSeparator)+current)
}

// StopPlaywright stops the global Playwright instance
func StopPlaywright() error {
	if pw != nil {
		return pw.Stop()
	}
	return nil
}

// getBrowserType maps string browser names to Playwright browser types
func getBrowserType(name string) (playwright.BrowserType, error) {
	p, err := getPlaywright()
	if err != nil {
		return nil, err
	}

	switch name {
	case "chrome", "chromium":
		return p.Chromium, nil
	case "firefox":
		return p.Firefox, nil
	case "webkit":
		return p.WebKit, nil
	case "msedge", "edge":
		return p.Chromium, nil // msedge also uses chromium
	default:
		return p.Chromium, nil
	}
}

// viewportSizeToPV converts session.ViewportSize to *playwright.Size
func viewportSizeToPV(vs *session.ViewportSize) *playwright.Size {
	if vs == nil {
		return nil
	}
	return &playwright.Size{
		Width:  vs.Width,
		Height: vs.Height,
	}
}
