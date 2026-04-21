package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/browserless/go-cli-browser/internal/daemon"
	"github.com/browserless/go-cli-browser/internal/output"
	"github.com/browserless/go-cli-browser/internal/session"
	"github.com/browserless/go-cli-browser/internal/snapshot"
	"github.com/spf13/cobra"
)

// goBackCmd represents the go-back command
var goBackCmd = &cobra.Command{
	Use:   "go-back",
	Short: "Navigate back",
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())

		// Try daemon mode first
		if daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()

				result, err := client.GoBack(cmd.GetSessionName())
				if err == nil && result.Success {
					snapshotResult, err := client.Snapshot(cmd.GetSessionName())
					if err == nil && snapshotResult.Success && snapshotResult.Snapshot != nil {
						fmt.Print(formatter.FormatSnapshot(daemonSnapshotToSnapshot(snapshotResult.Snapshot)))
					} else {
						fmt.Printf("### Page\n- Page URL: \n- Page Title: \n- Timestamp: %s\n\n### Elements\n",
							time.Now().Format(time.RFC3339))
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

		_, _ = page.GoBack()

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// goForwardCmd represents the go-forward command
var goForwardCmd = &cobra.Command{
	Use:   "go-forward",
	Short: "Navigate forward",
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())

		// Try daemon mode first
		if daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()

				result, err := client.GoForward(cmd.GetSessionName())
				if err == nil && result.Success {
					snapshotResult, err := client.Snapshot(cmd.GetSessionName())
					if err == nil && snapshotResult.Success && snapshotResult.Snapshot != nil {
						fmt.Print(formatter.FormatSnapshot(daemonSnapshotToSnapshot(snapshotResult.Snapshot)))
					} else {
						fmt.Printf("### Page\n- Page URL: \n- Page Title: \n- Timestamp: %s\n\n### Elements\n",
							time.Now().Format(time.RFC3339))
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

		_, _ = page.GoForward()

		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// reloadCmd represents the reload command
var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload the current page",
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())

		// Try daemon mode first
		if daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()

				result, err := client.Reload(cmd.GetSessionName())
				if err == nil && result.Success {
					snapshotResult, err := client.Snapshot(cmd.GetSessionName())
					if err == nil && snapshotResult.Success && snapshotResult.Snapshot != nil {
						fmt.Print(formatter.FormatSnapshot(daemonSnapshotToSnapshot(snapshotResult.Snapshot)))
					} else {
						fmt.Printf("### Page\n- Page URL: \n- Page Title: \n- Timestamp: %s\n\n### Elements\n",
							time.Now().Format(time.RFC3339))
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

		_, _ = page.Reload()

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
	cmd.RootCmd.AddCommand(goBackCmd)
	cmd.RootCmd.AddCommand(goForwardCmd)
	cmd.RootCmd.AddCommand(reloadCmd)
}

var _ = session.ModeLocal // suppress unused
