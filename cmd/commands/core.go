package commands

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/browserless/go-cli-browser/internal/browser"
	"github.com/browserless/go-cli-browser/internal/daemon"
	"github.com/browserless/go-cli-browser/internal/locator"
	"github.com/browserless/go-cli-browser/internal/output"
	"github.com/browserless/go-cli-browser/internal/session"
	"github.com/browserless/go-cli-browser/internal/snapshot"
	"github.com/playwright-community/playwright-go"
	"github.com/spf13/cobra"
)

var (
	snapshotFilename string
	snapshotDepth    int
	snapshotSelector string
)

// openCmd represents the open command
var openCmd = &cobra.Command{
	Use:   "open [url]",
	Short: "Open a browser and optionally navigate to URL",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())

		// When daemon is running and no special connection mode, use daemon
		if daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()

				opts := cmd.GetSessionOptions()
				sessionName := cmd.GetSessionName()

				url := ""
				if len(args) > 0 {
					var normalizeErr error
					url, normalizeErr = normalizeTargetURL(args[0])
					if normalizeErr != nil {
						return normalizeErr
					}
				}

				result, err := client.Open(sessionName, url, opts.Browser, opts.Headless)
				if err == nil && result.Success {
					// Get snapshot from daemon
					snapshotResult, err := client.Snapshot(sessionName)
					if err == nil && snapshotResult.Success && snapshotResult.Snapshot != nil {
						fmt.Print(formatter.FormatSnapshot(daemonSnapshotToSnapshot(snapshotResult.Snapshot)))
					} else {
						// Fallback: just print page info
						fmt.Printf("### Page\n- Page URL: %s\n- Page Title: %s\n- Timestamp: %s\n\n### Elements\n",
							result.Session.CurrentURL, result.Session.BrowserType, time.Now().Format(time.RFC3339))
					}
					return nil
				}
				// If daemon fails, fall through to local mode
			}
		}

		// Local mode (original implementation)
		sessionName := cmd.GetSessionName()
		opts := cmd.GetSessionOptions()
		pm := session.GetPersistenceManager()

		// Check if session already exists and is active
		var ctx playwright.BrowserContext
		var page playwright.Page

		if cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
			// Local mode - check for existing session
			state, stateErr := pm.Get(sessionName)
			if stateErr == nil && pm.IsActive(state) {
				// Reconnect to existing browser via CDP
				cdpURL := fmt.Sprintf("ws://localhost:%d", state.CDPPort)
				attachBrowser := browser.NewAttachBrowser()
				ctx, err := attachBrowser.ConnectViaCDP(cdpURL)
				if err == nil {
					pages := ctx.Pages()
					if len(pages) > 0 {
						page = pages[0]
					} else {
						page, _ = ctx.NewPage()
					}
					// Update session with reconnected context
					sess := cmd.GetOrCreateSession()
					sess.Context = ctx
					sess.Mode = session.ModeLocal
					sess.BrowserType = state.BrowserType
					mgr := session.GetManager()
					mgr.Set(sessionName, sess)
				}
			}
		}

		// If no reconnection happened, launch new browser
		if ctx == nil {
			var browserImpl browser.Browser

			if cmd.GetCDPURL() != "" || cmd.GetAttachExt() {
				browserImpl = browser.NewAttachBrowser()
			} else if cmd.GetRemoteURL() != "" {
				browserImpl = browser.NewRemoteBrowser()
			} else {
				browserImpl = browser.NewLocalBrowser()
			}

			sess := cmd.GetOrCreateSession()
			sess.Mode = session.ModeLocal
			sess.BrowserType = opts.Browser
			sess.Headless = opts.Headless
			sess.ProfileDir = opts.ProfileDir

			var err error

			// Launch or connect
			if cmd.GetCDPURL() != "" {
				sess.Mode = session.ModeAttached
				ctx, err = browserImpl.ConnectViaCDP(cmd.GetCDPURL())
			} else if cmd.GetAttachExt() {
				sess.Mode = session.ModeAttached
				ctx, err = browserImpl.ConnectViaCDP("ws://localhost:9222")
			} else if cmd.GetRemoteURL() != "" {
				sess.Mode = session.ModeRemote
				ctx, err = browserImpl.Connect(cmd.GetRemoteURL(), opts)
			} else {
				sess.Mode = session.ModeLocal
				result, launchErr := browserImpl.Launch(opts)
				if launchErr != nil {
					return launchErr
				}
				ctx = result.Context
				// Save session state for persistence
				pm.Save(&session.SessionState{
					Name:        sessionName,
					CDPPort:     result.CDPPort,
					Pid:         result.Pid,
					BrowserType: opts.Browser,
					Headless:    opts.Headless,
				})
			}

			if err != nil {
				return err
			}

			sess.Context = ctx

			// Create initial page
			page, err = ctx.NewPage()
			if err != nil {
				return err
			}
			sess.AddPage(page)

			// Save session
			mgr := session.GetManager()
			mgr.Set(sessionName, sess)
		}

		// Navigate if URL provided
		if len(args) > 0 {
			url, err := normalizeTargetURL(args[0])
			if err != nil {
				return err
			}
			if _, err := page.Goto(url, playwright.PageGotoOptions{
				Timeout: floatPtr(cmd.GetTimeout()),
			}); err != nil {
				return err
			}
		}

		// Take snapshot
		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// gotoCmd represents the goto command
var gotoCmd = &cobra.Command{
	Use:   "goto <url>",
	Short: "Navigate to URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		url, err := normalizeTargetURL(args[0])
		if err != nil {
			return err
		}

		// Try daemon mode first
		if daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()

				result, err := client.Goto(cmd.GetSessionName(), url)
				if err == nil && result.Success {
					snapshotResult, err := client.Snapshot(cmd.GetSessionName())
					if err == nil && snapshotResult.Success && snapshotResult.Snapshot != nil {
						fmt.Print(formatter.FormatSnapshot(daemonSnapshotToSnapshot(snapshotResult.Snapshot)))
					} else {
						fmt.Printf("### Page\n- Page URL: %s\n- Page Title: \n- Timestamp: %s\n\n### Elements\n",
							url, time.Now().Format(time.RFC3339))
					}
					return nil
				}
			}
		}

		// Local mode
		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		if _, err := page.Goto(url, playwright.PageGotoOptions{
			Timeout: floatPtr(cmd.GetTimeout()),
		}); err != nil {
			return err
		}

		// Take snapshot
		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// closeCmd represents the close command
var closeCmd = &cobra.Command{
	Use:   "close",
	Short: "Close the current session",
	RunE: func(c *cobra.Command, args []string) error {
		if daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()
				if result, err := client.CloseSession(cmd.GetSessionName()); err == nil && result.Success {
					fmt.Println(result.Message)
					return nil
				}
			}
		}

		mgr := session.GetManager()

		sess, err := mgr.Get(cmd.GetSessionName())
		if err != nil {
			// Session doesn't exist, nothing to close
			return nil
		}

		if sess.Context != nil {
			if err := sess.Context.Close(); err != nil {
				return err
			}
		}

		mgr.Delete(cmd.GetSessionName())
		session.GetPersistenceManager().Delete(cmd.GetSessionName())
		return nil
	},
}

// clickCmd represents the click command
var clickCmd = &cobra.Command{
	Use:   "click <locator>",
	Short: "Click an element",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Click(cmd.GetSessionName(), args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon click failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		locatorStr := args[0]
		resolver := locator.NewResolver(page)
		loc, err := resolver.Resolve(locatorStr)
		if err != nil {
			return err
		}

		if err := loc.Click(playwright.LocatorClickOptions{
			Timeout: floatPtr(cmd.GetTimeout()),
		}); err != nil {
			return err
		}

		// Take snapshot
		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// dblclickCmd represents the double click command
var dblclickCmd = &cobra.Command{
	Use:   "dblclick <locator>",
	Short: "Double-click an element",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.DblClick(cmd.GetSessionName(), args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon dblclick failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		locatorStr := args[0]
		resolver := locator.NewResolver(page)
		loc, err := resolver.Resolve(locatorStr)
		if err != nil {
			return err
		}

		if err := loc.Dblclick(playwright.LocatorDblclickOptions{
			Timeout: floatPtr(cmd.GetTimeout()),
		}); err != nil {
			return err
		}

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// typeCmd represents the type command
var typeCmd = &cobra.Command{
	Use:   "type <text>",
	Short: "Type text (presses keys on focused element)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		text := strings.Join(args, " ")
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Type(cmd.GetSessionName(), text)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon type failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		if err := page.Keyboard().Type(text, keyboardTypeOptions(0)); err != nil {
			return err
		}

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// fillCmd represents the fill command
var fillCmd = &cobra.Command{
	Use:   "fill <locator> <text>",
	Short: "Fill an input element",
	Args:  cobra.ExactArgs(2),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			submit, _ := c.Flags().GetBool("submit")
			result, err := client.Fill(cmd.GetSessionName(), args[0], args[1], submit)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon fill failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		locatorStr := args[0]
		text := args[1]
		resolver := locator.NewResolver(page)
		loc, err := resolver.Resolve(locatorStr)
		if err != nil {
			return err
		}

		if err := loc.Fill(text, locatorFillOptions()); err != nil {
			return err
		}

		// Handle --submit flag
		submit, _ := c.Flags().GetBool("submit")
		if submit {
			if err := loc.Press("Enter", locatorPressOptions()); err != nil {
				return err
			}
		}

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

var (
	submitFlag bool
)

// hoverCmd represents the hover command
var hoverCmd = &cobra.Command{
	Use:   "hover <locator>",
	Short: "Hover over an element",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Hover(cmd.GetSessionName(), args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon hover failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		locatorStr := args[0]
		resolver := locator.NewResolver(page)
		loc, err := resolver.Resolve(locatorStr)
		if err != nil {
			return err
		}

		if err := loc.Hover(locatorHoverOptions(cmd.GetTimeout())); err != nil {
			return err
		}

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// selectCmd represents the select command
var selectCmd = &cobra.Command{
	Use:   "select <locator> <value>",
	Short: "Select an option from a dropdown",
	Args:  cobra.ExactArgs(2),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Select(cmd.GetSessionName(), args[0], args[1])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon select failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		locatorStr := args[0]
		value := args[1]
		resolver := locator.NewResolver(page)
		loc, err := resolver.Resolve(locatorStr)
		if err != nil {
			return err
		}

		values := []string{value}
		selected, err := loc.SelectOption(playwright.SelectOptionValues{Values: &values}, locatorSelectOptionOptions())
		_ = selected
		if err != nil {
			return err
		}

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// dragCmd represents the drag command
var dragCmd = &cobra.Command{
	Use:   "drag <sourceLocator> <targetLocator>",
	Short: "Drag source element to target element",
	Args:  cobra.ExactArgs(2),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Drag(cmd.GetSessionName(), args[0], args[1])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon drag failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		sourceStr := args[0]
		targetStr := args[1]
		resolver := locator.NewResolver(page)

		sourceLoc, err := resolver.Resolve(sourceStr)
		if err != nil {
			return err
		}

		targetLoc, err := resolver.Resolve(targetStr)
		if err != nil {
			return err
		}

		if err := sourceLoc.DragTo(targetLoc, locatorDragToOptions()); err != nil {
			return err
		}

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload <filePath>",
	Short: "Upload a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Upload(cmd.GetSessionName(), "", args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon upload failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		filePath := args[0]
		if err := page.SetInputFiles("input[type=file]", filePath); err != nil {
			return err
		}

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check <locator>",
	Short: "Check a checkbox or radio button",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Check(cmd.GetSessionName(), args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon check failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		locatorStr := args[0]
		resolver := locator.NewResolver(page)
		loc, err := resolver.Resolve(locatorStr)
		if err != nil {
			return err
		}

		if err := loc.Check(playwright.LocatorCheckOptions{
			Timeout: floatPtr(cmd.GetTimeout()),
		}); err != nil {
			return err
		}

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// uncheckCmd represents the uncheck command
var uncheckCmd = &cobra.Command{
	Use:   "uncheck <locator>",
	Short: "Uncheck a checkbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Uncheck(cmd.GetSessionName(), args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon uncheck failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		locatorStr := args[0]
		resolver := locator.NewResolver(page)
		loc, err := resolver.Resolve(locatorStr)
		if err != nil {
			return err
		}

		if err := loc.Uncheck(playwright.LocatorUncheckOptions{
			Timeout: floatPtr(cmd.GetTimeout()),
		}); err != nil {
			return err
		}

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// snapshotCmd represents the snapshot command
var snapshotCmd = &cobra.Command{
	Use:   "snapshot [selector]",
	Short: "Take a snapshot of the page",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())

		// Try daemon mode first
		if daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()

				snapshotResult, err := client.Snapshot(cmd.GetSessionName())
				if err == nil && snapshotResult.Success && snapshotResult.Snapshot != nil {
					fmt.Print(formatter.FormatSnapshot(daemonSnapshotToSnapshot(snapshotResult.Snapshot)))
					return nil
				}
			}
		}

		// Local mode
		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		snap, err := snapshot.GenerateSnapshot(page, snapshotDepth)
		if err != nil {
			return err
		}

		if snapshotFilename != "" {
			// Save to file
			data := formatter.FormatSnapshot(snap)
			if err := os.WriteFile(snapshotFilename, []byte(data), 0644); err != nil {
				return err
			}
			fmt.Printf("Snapshot saved to %s\n", snapshotFilename)
			return nil
		}

		fmt.Print(formatter.FormatSnapshot(snap))
		return nil
	},
}

// evalCmd represents the eval command
var evalCmd = &cobra.Command{
	Use:   "eval <expression>",
	Short: "Evaluate JavaScript expression",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Eval(cmd.GetSessionName(), args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon eval failed: %s", result.Message)
			}
			fmt.Print(FormatEvalResult(result.Value))
			return nil
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		expression := args[0]
		result, err := page.Evaluate(expression, nil)
		if err != nil {
			return err
		}

		fmt.Print(FormatEvalResult(result))
		return nil
	},
}

// dialogCmd represents the dialog-accept command
var dialogAcceptCmd = &cobra.Command{
	Use:   "dialog-accept [text]",
	Short: "Accept a dialog",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		text := ""
		if len(args) > 0 {
			text = args[0]
		}

		// Set up dialog handler
		page.OnDialog(func(dialog playwright.Dialog) {
			if text != "" {
				dialog.Accept(text)
			} else {
				dialog.Accept()
			}
		})

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// dialogDismissCmd represents the dialog-dismiss command
var dialogDismissCmd = &cobra.Command{
	Use:   "dialog-dismiss",
	Short: "Dismiss a dialog",
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		page.OnDialog(func(dialog playwright.Dialog) {
			dialog.Dismiss()
		})

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// resizeCmd represents the resize command
var resizeCmd = &cobra.Command{
	Use:   "resize <width> <height>",
	Short: "Resize the viewport",
	Args:  cobra.ExactArgs(2),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		width, height, err := parseViewportSize(args[0], args[1])
		if err != nil {
			return err
		}
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Resize(cmd.GetSessionName(), width, height)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon resize failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		if err := page.SetViewportSize(width, height); err != nil {
			return err
		}

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

func init() {
	// Register all core commands
	cmd.RootCmd.AddCommand(openCmd)
	cmd.RootCmd.AddCommand(gotoCmd)
	cmd.RootCmd.AddCommand(closeCmd)
	cmd.RootCmd.AddCommand(clickCmd)
	cmd.RootCmd.AddCommand(dblclickCmd)
	cmd.RootCmd.AddCommand(typeCmd)
	cmd.RootCmd.AddCommand(fillCmd)
	cmd.RootCmd.AddCommand(hoverCmd)
	cmd.RootCmd.AddCommand(selectCmd)
	cmd.RootCmd.AddCommand(dragCmd)
	cmd.RootCmd.AddCommand(uploadCmd)
	cmd.RootCmd.AddCommand(checkCmd)
	cmd.RootCmd.AddCommand(uncheckCmd)
	cmd.RootCmd.AddCommand(snapshotCmd)
	cmd.RootCmd.AddCommand(evalCmd)
	cmd.RootCmd.AddCommand(dialogAcceptCmd)
	cmd.RootCmd.AddCommand(dialogDismissCmd)
	cmd.RootCmd.AddCommand(resizeCmd)

	// Add flags
	fillCmd.Flags().BoolVar(&submitFlag, "submit", false, "Press Enter after filling")
	snapshotCmd.Flags().StringVar(&snapshotFilename, "filename", "", "Save snapshot to file")
	snapshotCmd.Flags().IntVar(&snapshotDepth, "depth", 3, "Snapshot depth")
}

// FormatEvalResult formats an eval result
func FormatEvalResult(result interface{}) string {
	return fmt.Sprintf("Result: %v\n", result)
}

func daemonMode() bool {
	return daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == ""
}

func printDaemonSnapshot(formatter *output.Formatter, client *daemon.Client, sessionName string) error {
	snapshotResult, err := client.Snapshot(sessionName)
	if err != nil {
		return err
	}
	if !snapshotResult.Success || snapshotResult.Snapshot == nil {
		return fmt.Errorf("daemon snapshot failed")
	}
	fmt.Print(formatter.FormatSnapshot(daemonSnapshotToSnapshot(snapshotResult.Snapshot)))
	return nil
}

func parseViewportSize(widthArg, heightArg string) (int, int, error) {
	var width, height int
	if _, err := fmt.Sscanf(widthArg, "%d", &width); err != nil {
		return 0, 0, fmt.Errorf("invalid width %q", widthArg)
	}
	if _, err := fmt.Sscanf(heightArg, "%d", &height); err != nil {
		return 0, 0, fmt.Errorf("invalid height %q", heightArg)
	}
	return width, height, nil
}

func normalizeTargetURL(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" {
		return raw, nil
	}
	if info, err := os.Stat(raw); err == nil && !info.IsDir() {
		abs, err := filepath.Abs(raw)
		if err != nil {
			return "", err
		}
		return (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String(), nil
	}
	if strings.Contains(raw, ".") && !strings.ContainsAny(raw, "/\\ ") {
		return "https://" + raw, nil
	}
	return raw, nil
}

func daemonSnapshotToSnapshot(info *daemon.SnapshotInfo) *snapshot.Snapshot {
	ts := info.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	return &snapshot.Snapshot{
		URL:       info.URL,
		Title:     info.Title,
		Timestamp: ts,
		Elements:  info.Elements,
	}
}

var _ = time.Second // suppress unused import
