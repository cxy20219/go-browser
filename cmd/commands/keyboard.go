package commands

import (
	"fmt"
	"os"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/browserless/go-cli-browser/internal/daemon"
	"github.com/browserless/go-cli-browser/internal/output"
	"github.com/browserless/go-cli-browser/internal/snapshot"
	"github.com/playwright-community/playwright-go"
	"github.com/spf13/cobra"
)

// pressCmd represents the press command
var pressCmd = &cobra.Command{
	Use:   "press <key>",
	Short: "Press a keyboard key",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		key := args[0]
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.Press(cmd.GetSessionName(), key)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon press failed: %s", result.Message)
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

		if err := page.Keyboard().Press(key, playwright.KeyboardPressOptions{}); err != nil {
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

// keydownCmd represents the keydown command
var keydownCmd = &cobra.Command{
	Use:   "keydown <key>",
	Short: "Press and hold a keyboard key",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		key := args[0]
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.KeyDown(cmd.GetSessionName(), key)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon keydown failed: %s", result.Message)
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

		if err := page.Keyboard().Down(key); err != nil {
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

// keyupCmd represents the keyup command
var keyupCmd = &cobra.Command{
	Use:   "keyup <key>",
	Short: "Release a keyboard key",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		key := args[0]
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.KeyUp(cmd.GetSessionName(), key)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon keyup failed: %s", result.Message)
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

		if err := page.Keyboard().Up(key); err != nil {
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
	cmd.RootCmd.AddCommand(pressCmd)
	cmd.RootCmd.AddCommand(keydownCmd)
	cmd.RootCmd.AddCommand(keyupCmd)
}
