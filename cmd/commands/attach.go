package commands

import (
	"fmt"
	"os"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/browserless/go-cli-browser/internal/browser"
	"github.com/browserless/go-cli-browser/internal/output"
	"github.com/browserless/go-cli-browser/internal/session"
	"github.com/browserless/go-cli-browser/internal/snapshot"
	"github.com/spf13/cobra"
)

var (
	cdpChannel string
)

// attachCmd represents the attach command
var attachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Attach to a running browser",
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())

		// Get CDP URL
		var cdpURL string
		var err error

		if cmd.GetCDPURL() != "" {
			cdpURL = cmd.GetCDPURL()
		} else if cdpChannel != "" {
			cdpURL, err = browser.GetCDPURLForChannel(cdpChannel)
			if err != nil {
				return err
			}
		} else if cmd.GetAttachExt() {
			cdpURL = "ws://localhost:9222"
		} else {
			return fmt.Errorf("--cdp or --extension flag required")
		}

		// Create attach browser
		attachBrowser := browser.NewAttachBrowser()
		ctx, err := attachBrowser.ConnectViaCDP(cdpURL)
		if err != nil {
			return err
		}

		// Create or get session
		sess := cmd.GetOrCreateSession()
		sess.Mode = session.ModeAttached
		sess.Context = ctx

		// Create a new page if none exist
		page, err := ctx.NewPage()
		if err != nil {
			return err
		}
		sess.AddPage(page)

		// Save session
		mgr := session.GetManager()
		mgr.Set(cmd.GetSessionName(), sess)

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

func init() {
	cmd.RootCmd.AddCommand(attachCmd)

	attachCmd.Flags().StringVar(&cdpChannel, "channel", "", "Browser channel (chrome, msedge)")
}
