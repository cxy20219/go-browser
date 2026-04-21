package cmd

import (
	"fmt"
	"os"

	"github.com/browserless/go-cli-browser/internal/session"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	sessionName string
	browserType string
	headless    bool
	headed      bool
	remoteURL   string
	cdpURL      string
	attachExt   bool
	raw         bool
	timeout     int

	// Persistent flags for open command
	persistent bool
	profileDir string
	configFile string
)

// RootCmd is the root command
var RootCmd = &cobra.Command{
	Use:   "go-browser",
	Short: "Browser automation CLI tool",
	Long: `A browser automation CLI tool supporting local and remote browser connections.
Use with Browserless cloud for scalable browser automation.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if flag := cmd.Flags().Lookup("headed"); flag != nil && flag.Changed {
			headless = false
		}
		if flag := cmd.InheritedFlags().Lookup("headed"); flag != nil && flag.Changed {
			headless = false
		}
		// Initialize session manager
		session.Init()
		return nil
	},
}

// Execute executes the root command
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global persistent flags
	RootCmd.PersistentFlags().StringVarP(&sessionName, "session", "s", "default", "Session name")
	RootCmd.PersistentFlags().StringVar(&browserType, "browser", "chromium", "Browser type: chrome, firefox, webkit, msedge")
	RootCmd.PersistentFlags().BoolVar(&headless, "headless", true, "Run in headless mode")
	RootCmd.PersistentFlags().BoolVar(&headed, "headed", false, "Run in headed mode (alias for --headless=false)")
	RootCmd.PersistentFlags().StringVar(&remoteURL, "remote", "", "Remote Browserless URL (wss://...)")
	RootCmd.PersistentFlags().StringVar(&cdpURL, "cdp", "", "CDP endpoint URL (ws://localhost:9222)")
	RootCmd.PersistentFlags().BoolVar(&attachExt, "extension", false, "Attach via browser extension")
	RootCmd.PersistentFlags().BoolVar(&raw, "raw", false, "Raw output (no formatting)")
	RootCmd.PersistentFlags().IntVar(&timeout, "timeout", 30000, "Timeout in milliseconds")

	// Open command flags
	RootCmd.PersistentFlags().BoolVar(&persistent, "persistent", false, "Use persistent profile")
	RootCmd.PersistentFlags().StringVar(&profileDir, "profile", "", "Profile directory for persistent mode")
	RootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file path")

	// Bind flags to viper for env var support
	// viper.BindPFlag("session", RootCmd.PersistentFlags().Lookup("session"))
	// viper.BindPFlag("browser", RootCmd.PersistentFlags().Lookup("browser"))
	// viper.BindPFlag("headless", RootCmd.PersistentFlags().Lookup("headless"))
	// viper.BindPFlag("remote", RootCmd.PersistentFlags().Lookup("remote"))
}

// GetSessionOptions returns session options from flags
func GetSessionOptions() *session.SessionOptions {
	opts := &session.SessionOptions{
		Browser:    browserType,
		Headless:   headless,
		Persistent: persistent,
		ProfileDir: profileDir,
	}

	if remoteURL != "" {
		opts.RemoteURL = remoteURL
	}
	if cdpURL != "" {
		opts.CDPURL = cdpURL
	}
	if attachExt {
		opts.AttachExt = attachExt
	}

	return opts
}

// GetSession returns the session for the current session name
func GetSession() (*session.Session, error) {
	mgr := session.GetManager()
	return mgr.Get(sessionName)
}

// GetOrCreateSession returns or creates a session
func GetOrCreateSession() *session.Session {
	mgr := session.GetManager()
	return mgr.GetOrCreate(sessionName)
}

// Getters for exported access from commands package
func GetSessionName() string { return sessionName }
func GetRaw() bool           { return raw }
func GetCDPURL() string      { return cdpURL }
func GetAttachExt() bool     { return attachExt }
func GetRemoteURL() string   { return remoteURL }
func GetTimeout() int        { return timeout }
func GetBrowserType() string { return browserType }
