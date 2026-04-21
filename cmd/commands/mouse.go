package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/browserless/go-cli-browser/internal/daemon"
	"github.com/browserless/go-cli-browser/internal/output"
	"github.com/browserless/go-cli-browser/internal/snapshot"
	"github.com/playwright-community/playwright-go"
	"github.com/spf13/cobra"
)

// mousemoveCmd represents the mousemove command
var mousemoveCmd = &cobra.Command{
	Use:   "mousemove <x> <y>",
	Short: "Move mouse to position",
	Args:  cobra.ExactArgs(2),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		x, y, err := parseMousePair(args[0], args[1])
		if err != nil {
			return err
		}
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.MouseMove(cmd.GetSessionName(), x, y)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon mousemove failed: %s", result.Message)
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

		if err := page.Mouse().Move(x, y); err != nil {
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

// mousedownCmd represents the mousedown command
var mousedownCmd = &cobra.Command{
	Use:   "mousedown [button]",
	Short: "Press mouse button (left, right)",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		button := mouseButtonArg(args)
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.MouseDown(cmd.GetSessionName(), button)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon mousedown failed: %s", result.Message)
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

		options, err := localMouseDownOptions(button)
		if err != nil {
			return err
		}

		if err := page.Mouse().Down(options); err != nil {
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

// mouseupCmd represents the mouseup command
var mouseupCmd = &cobra.Command{
	Use:   "mouseup [button]",
	Short: "Release mouse button (left, right)",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		button := mouseButtonArg(args)
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.MouseUp(cmd.GetSessionName(), button)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon mouseup failed: %s", result.Message)
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

		options, err := localMouseUpOptions(button)
		if err != nil {
			return err
		}

		if err := page.Mouse().Up(options); err != nil {
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

// mousewheelCmd represents the mousewheel command
var mousewheelCmd = &cobra.Command{
	Use:   "mousewheel <deltaX> <deltaY>",
	Short: "Scroll mouse wheel",
	Args:  cobra.ExactArgs(2),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		deltaX, deltaY, err := parseMousePair(args[0], args[1])
		if err != nil {
			return err
		}
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.MouseWheel(cmd.GetSessionName(), deltaX, deltaY)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon mousewheel failed: %s", result.Message)
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

		if err := page.Mouse().Wheel(deltaX, deltaY); err != nil {
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
	cmd.RootCmd.AddCommand(mousemoveCmd)
	cmd.RootCmd.AddCommand(mousedownCmd)
	cmd.RootCmd.AddCommand(mouseupCmd)
	cmd.RootCmd.AddCommand(mousewheelCmd)
}

func parseMousePair(firstArg, secondArg string) (float64, float64, error) {
	first, err := strconv.ParseFloat(firstArg, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid coordinate or delta %q", firstArg)
	}
	second, err := strconv.ParseFloat(secondArg, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid coordinate or delta %q", secondArg)
	}
	return first, second, nil
}

func mouseButtonArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func localMouseDownOptions(button string) (playwright.MouseDownOptions, error) {
	switch strings.ToLower(strings.TrimSpace(button)) {
	case "", "left":
		return playwright.MouseDownOptions{}, nil
	case "right":
		return playwright.MouseDownOptions{Button: playwright.MouseButtonRight}, nil
	case "middle":
		return playwright.MouseDownOptions{Button: playwright.MouseButtonMiddle}, nil
	default:
		return playwright.MouseDownOptions{}, fmt.Errorf("unsupported mouse button %q", button)
	}
}

func localMouseUpOptions(button string) (playwright.MouseUpOptions, error) {
	switch strings.ToLower(strings.TrimSpace(button)) {
	case "", "left":
		return playwright.MouseUpOptions{}, nil
	case "right":
		return playwright.MouseUpOptions{Button: playwright.MouseButtonRight}, nil
	case "middle":
		return playwright.MouseUpOptions{Button: playwright.MouseButtonMiddle}, nil
	default:
		return playwright.MouseUpOptions{}, fmt.Errorf("unsupported mouse button %q", button)
	}
}
