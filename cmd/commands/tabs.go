package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/browserless/go-cli-browser/internal/daemon"
	"github.com/browserless/go-cli-browser/internal/output"
	"github.com/browserless/go-cli-browser/internal/snapshot"
	"github.com/playwright-community/playwright-go"
	"github.com/spf13/cobra"
)

// tabListCmd represents the tab-list command
var tabListCmd = &cobra.Command{
	Use:   "tab-list",
	Short: "List all tabs",
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.TabList(cmd.GetSessionName())
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon tab-list failed: %s", result.Message)
			}
			printDaemonTabs(result.Tabs)
			return nil
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		if len(sess.Pages) == 0 {
			fmt.Println("No tabs open")
			return nil
		}

		for i, page := range sess.Pages {
			url := page.URL()
			title, _ := page.Title()
			marker := " "
			if i == sess.CurrentPage {
				marker = "*"
			}
			if cmd.GetRaw() {
				fmt.Printf("%s%d | %s | %s\n", marker, i, url, title)
			} else {
				fmt.Printf("%s Tab %d: %s (%s)\n", marker, i, title, url)
			}
		}

		return nil
	},
}

// tabNewCmd represents the tab-new command
var tabNewCmd = &cobra.Command{
	Use:   "tab-new [url]",
	Short: "Open a new tab",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		url := ""
		if len(args) > 0 {
			var err error
			url, err = normalizeTargetURL(args[0])
			if err != nil {
				return err
			}
		}
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.TabNew(cmd.GetSessionName(), url)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon tab-new failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.Context.NewPage()
		if err != nil {
			return err
		}

		sess.AddPage(page)

		// Navigate if URL provided
		if url != "" {
			if _, err := page.Goto(url, playwright.PageGotoOptions{
				Timeout: floatPtr(cmd.GetTimeout()),
			}); err != nil {
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

// tabCloseCmd represents the tab-close command
var tabCloseCmd = &cobra.Command{
	Use:   "tab-close [index]",
	Short: "Close a tab by index",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		index := -1
		if len(args) > 0 {
			var err error
			index, err = parseTabIndex(args[0])
			if err != nil {
				return err
			}
		}
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.TabClose(cmd.GetSessionName(), index)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon tab-close failed: %s", result.Message)
			}
			if result.Session != nil && result.Session.PageCount == 0 {
				fmt.Println(result.Message)
				return nil
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		if index < 0 {
			index = sess.CurrentPage
		}

		if index < 0 || index >= len(sess.Pages) {
			return fmt.Errorf("invalid tab index %d", index)
		}

		page := sess.Pages[index]
		if err := page.Close(); err != nil {
			return err
		}

		sess.RemovePage(index)

		if len(sess.Pages) > 0 {
			snap, err := snapshot.GenerateSnapshot(sess.Pages[sess.CurrentPage], 3)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
			} else {
				fmt.Print(formatter.FormatSnapshot(snap))
			}
		} else {
			fmt.Println("All tabs closed")
		}

		return nil
	},
}

// tabSelectCmd represents the tab-select command
var tabSelectCmd = &cobra.Command{
	Use:   "tab-select <index>",
	Short: "Select a tab by index",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		formatter := output.NewFormatter(cmd.GetRaw())
		index, err := parseTabIndex(args[0])
		if err != nil {
			return err
		}
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.TabSelect(cmd.GetSessionName(), index)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon tab-select failed: %s", result.Message)
			}
			return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		if err := sess.SelectPage(index); err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
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

func init() {
	cmd.RootCmd.AddCommand(tabListCmd)
	cmd.RootCmd.AddCommand(tabNewCmd)
	cmd.RootCmd.AddCommand(tabCloseCmd)
	cmd.RootCmd.AddCommand(tabSelectCmd)
}

func printDaemonTabs(tabs []daemon.TabInfo) {
	if len(tabs) == 0 {
		fmt.Println("No tabs open")
		return
	}
	for _, tab := range tabs {
		marker := " "
		if tab.Current {
			marker = "*"
		}
		if cmd.GetRaw() {
			fmt.Printf("%s%d | %s | %s\n", marker, tab.Index, tab.URL, tab.Title)
		} else {
			fmt.Printf("%s Tab %d: %s (%s)\n", marker, tab.Index, tab.Title, tab.URL)
		}
	}
}

func parseTabIndex(raw string) (int, error) {
	index, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid tab index %q", raw)
	}
	return index, nil
}
