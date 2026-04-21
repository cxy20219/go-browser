package commands

import (
	"fmt"
	"os"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/browserless/go-cli-browser/internal/daemon"
	"github.com/browserless/go-cli-browser/internal/session"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all browser sessions",
	RunE: func(c *cobra.Command, args []string) error {
		if daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()
				result, err := client.ListSessions()
				if err == nil && result.Success {
					if len(result.Sessions) == 0 {
						fmt.Println("No active sessions")
						return nil
					}
					for _, name := range result.Sessions {
						if cmd.GetRaw() {
							fmt.Println(name)
						} else {
							fmt.Printf("* Session: %s (status=running)\n", name)
						}
					}
					return nil
				}
			}
		}

		pm := session.GetPersistenceManager()
		names := pm.List()

		if len(names) == 0 {
			fmt.Println("No active sessions")
			return nil
		}

		for _, name := range names {
			state, err := pm.Get(name)
			if err != nil {
				continue
			}
			active := pm.IsActive(state)
			if !active {
				// Clean up inactive session
				pm.Delete(name)
				continue
			}
			status := "running"
			if cmd.GetRaw() {
				fmt.Printf("Session: %s (mode=%s, pid=%d, cdp_port=%d, status=%s)\n",
					state.Name, state.BrowserType, state.Pid, state.CDPPort, status)
			} else {
				fmt.Printf("* Session: %s (mode=%s, pid=%d, cdp_port=%d, status=%s)\n",
					state.Name, state.BrowserType, state.Pid, state.CDPPort, status)
			}
		}

		return nil
	},
}

// sessionCloseCmd represents the close command
var sessionCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Close a browser session",
	RunE: func(c *cobra.Command, args []string) error {
		pm := session.GetPersistenceManager()
		name := cmd.GetSessionName()

		state, err := pm.Get(name)
		if err != nil {
			// Session doesn't exist, nothing to close
			return nil
		}

		// Try to kill the process
		proc, _ := os.FindProcess(state.Pid)
		if proc != nil {
			proc.Kill()
		}

		// Delete persisted state
		pm.Delete(name)
		fmt.Printf("Session %s closed\n", name)
		return nil
	},
}

// closeAllCmd represents the close-all command
var closeAllCmd = &cobra.Command{
	Use:   "close-all",
	Short: "Close all browser sessions",
	RunE: func(c *cobra.Command, args []string) error {
		pm := session.GetPersistenceManager()
		names := pm.List()

		for _, name := range names {
			state, _ := pm.Get(name)
			if state != nil {
				if proc, err := os.FindProcess(state.Pid); err == nil {
					proc.Kill()
				}
			}
			pm.Delete(name)
		}

		fmt.Println("All sessions closed")
		return nil
	},
}

// killAllCmd represents the kill-all command
var killAllCmd = &cobra.Command{
	Use:   "kill-all",
	Short: "Forcefully kill all browser processes",
	RunE: func(c *cobra.Command, args []string) error {
		pm := session.GetPersistenceManager()
		names := pm.List()

		for _, name := range names {
			state, _ := pm.Get(name)
			if state != nil {
				if proc, err := os.FindProcess(state.Pid); err == nil {
					proc.Kill()
				}
			}
			pm.Delete(name)
		}

		fmt.Println("All browser processes killed")
		return nil
	},
}

func init() {
	cmd.RootCmd.AddCommand(listCmd)
	cmd.RootCmd.AddCommand(closeAllCmd)
	cmd.RootCmd.AddCommand(killAllCmd)
}
