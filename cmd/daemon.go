package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/browserless/go-cli-browser/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	daemonHeaded bool
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the browser daemon",
	Long: `Start the browser daemon in the background.
The daemon maintains browser sessions and allows subsequent commands
to reuse the same browser instance, preserving state (cookies, tabs, etc.).`,
	RunE: func(c *cobra.Command, args []string) error {
		// Check if daemon is already running
		if daemon.IsDaemonRunning() {
			fmt.Println("Daemon is already running")
			return nil
		}

		// Start daemon in background
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		// Build args for daemon mode
		daemonArgs := []string{"daemon"}
		if daemonHeaded {
			daemonArgs = append(daemonArgs, "--headed")
		}

		// Start process
		cmd := exec.Command(exePath, daemonArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		configureDaemonProcess(cmd)

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}

		fmt.Printf("Daemon started (PID: %d)\n", cmd.Process.Pid)

		// Wait for daemon to be ready (Playwright initialization takes time)
		for i := 0; i < 30; i++ {
			time.Sleep(200 * time.Millisecond)
			if daemon.IsDaemonRunning() {
				fmt.Println("Daemon is ready")
				return nil
			}
		}

		fmt.Println("Warning: Daemon may not have started properly")
		return nil
	},
}

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the browser daemon",
	Long:  `Stop the browser daemon and close all browser sessions.`,
	RunE: func(c *cobra.Command, args []string) error {
		if !daemon.IsDaemonRunning() {
			fmt.Println("Daemon is not running")
			return nil
		}

		// Connect to daemon and stop it
		client, err := daemon.NewClient()
		if err != nil {
			// Fallback: kill via process info
			info, infoErr := daemon.GetDaemonInfo()
			if infoErr != nil {
				return fmt.Errorf("daemon not responding and could not get info: %w", infoErr)
			}
			proc, _ := os.FindProcess(info.PID)
			if proc != nil {
				proc.Kill()
			}
			fmt.Println("Daemon stopped (killed)")
			return nil
		}
		defer client.Close()

		// Send stop request
		result, err := client.Stop()
		if err != nil {
			// Fallback: kill via process info
			info, _ := daemon.GetDaemonInfo()
			if info != nil {
				proc, _ := os.FindProcess(info.PID)
				if proc != nil {
					proc.Kill()
				}
			}
			fmt.Printf("Daemon stopped (error: %v)\n", err)
			return nil
		}

		if result.Success {
			fmt.Println("Daemon stopped")
		} else {
			fmt.Printf("Daemon stop failed: %s\n", result.Message)
		}

		return nil
	},
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Long:  `Show the status of the browser daemon.`,
	RunE: func(c *cobra.Command, args []string) error {
		if !daemon.IsDaemonRunning() {
			fmt.Println("Daemon is not running")
			return nil
		}

		client, err := daemon.NewClient()
		if err != nil {
			fmt.Printf("Daemon is running but cannot connect: %v\n", err)
			return nil
		}
		defer client.Close()

		info, err := client.Status()
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}

		fmt.Printf("Daemon Status:\n")
		fmt.Printf("  PID: %d\n", info.PID)
		fmt.Printf("  Socket: %s\n", info.SocketPath)
		fmt.Printf("  Sessions: %d\n", len(info.Sessions))
		fmt.Printf("  Version: %s\n", info.Version)

		if len(info.Sessions) > 0 {
			fmt.Println("  Active sessions:")
			for _, s := range info.Sessions {
				fmt.Printf("    - %s\n", s)
			}
		}

		return nil
	},
}

// daemonCmd is the internal daemon command (runs the server)
var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Internal command to run daemon server",
	Hidden: true,
	RunE: func(c *cobra.Command, args []string) error {
		// headed mode means headless=false
		headless := !daemonHeaded
		server, err := daemon.NewServer(headless)
		if err != nil {
			return fmt.Errorf("failed to create server: %w", err)
		}

		if err := server.Start(); err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}

		fmt.Printf("Daemon listening on %s\n", daemon.GetSocketPath())

		// Wait for interrupt signal or stop request
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		// Wait for either signal or server's stop channel
		select {
		case <-sigCh:
		case <-server.GetStopCh():
		}

		// Clean shutdown
		server.Stop()
		return nil
	},
}

func init() {
	RootCmd.AddCommand(startCmd)
	RootCmd.AddCommand(stopCmd)
	RootCmd.AddCommand(statusCmd)
	RootCmd.AddCommand(daemonCmd)

	startCmd.Flags().BoolVarP(&daemonHeaded, "headed", "", false, "Run browser in headed mode (default: headless)")
	daemonCmd.Flags().BoolVar(&daemonHeaded, "headed", false, "Run browser in headed mode")
	daemonCmd.Flags().Lookup("headed").Hidden = true
}
