package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/browserless/go-cli-browser/internal/daemon"
	"github.com/playwright-community/playwright-go"
	"github.com/spf13/cobra"
)

var (
	cookieDomain   string
	cookieHTTPOnly bool
	cookieSecure   bool
	stateFilename  string
)

// stateSaveCmd represents the state-save command
var stateSaveCmd = &cobra.Command{
	Use:   "state-save [filename]",
	Short: "Save browser state",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		filename := ""
		if len(args) > 0 {
			filename = args[0]
		}
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.StateSave(cmd.GetSessionName(), filename)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon state-save failed: %s", result.Message)
			}
			fmt.Println(result.Message)
			return nil
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		ctx := sess.Context
		if ctx == nil {
			return fmt.Errorf("no browser context")
		}

		if filename == "" {
			filename = fmt.Sprintf("storage-state-%d.json", os.Getpid())
		}

		state, err := ctx.StorageState()
		if err != nil {
			return fmt.Errorf("failed to get storage state: %w", err)
		}

		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal state: %w", err)
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			return fmt.Errorf("failed to write state file: %w", err)
		}

		fmt.Printf("State saved to %s\n", filename)
		return nil
	},
}

// stateLoadCmd represents the state-load command
var stateLoadCmd = &cobra.Command{
	Use:   "state-load [filename]",
	Short: "Load browser state",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		filename := ""
		if len(args) > 0 {
			filename = args[0]
		}
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.StateLoad(cmd.GetSessionName(), filename)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon state-load failed: %s", result.Message)
			}
			fmt.Println(result.Message)
			return nil
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		ctx := sess.Context
		if ctx == nil {
			return fmt.Errorf("no browser context")
		}

		if filename == "" {
			return fmt.Errorf("filename required")
		}

		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read state file: %w", err)
		}

		var state playwright.StorageState
		if err := json.Unmarshal(data, &state); err != nil {
			return fmt.Errorf("failed to parse state file: %w", err)
		}

		// Convert to OptionalCookie for AddCookies
		cookies := make([]playwright.OptionalCookie, len(state.Cookies))
		for i, c := range state.Cookies {
			cookies[i] = playwright.OptionalCookie{
				Name:  c.Name,
				Value: c.Value,
			}
			if c.Domain != "" {
				cookies[i].Domain = &c.Domain
			}
			if c.Path != "" {
				cookies[i].Path = &c.Path
			}
			if c.Expires != 0 {
				cookies[i].Expires = &c.Expires
			}
			if c.HttpOnly {
				cookies[i].HttpOnly = &c.HttpOnly
			}
			if c.Secure {
				cookies[i].Secure = &c.Secure
			}
			if c.SameSite != nil && *c.SameSite != "" {
				cookies[i].SameSite = c.SameSite
			}
		}

		if err := ctx.AddCookies(cookies); err != nil {
			return fmt.Errorf("failed to add cookies: %w", err)
		}

		fmt.Printf("State loaded from %s\n", filename)
		return nil
	},
}

// cookieListCmd represents the cookie-list command
var cookieListCmd = &cobra.Command{
	Use:   "cookie-list",
	Short: "List cookies",
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.CookieList(cmd.GetSessionName())
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon cookie-list failed: %s", result.Message)
			}
			if len(result.Cookies) == 0 {
				fmt.Println("No cookies")
				return nil
			}
			for _, c := range result.Cookies {
				fmt.Printf("%s=%s; Domain=%s; Path=%s\n", c.Name, c.Value, c.Domain, c.Path)
			}
			return nil
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		ctx := sess.Context
		if ctx == nil {
			return fmt.Errorf("no browser context")
		}

		cookies, err := ctx.Cookies()
		if err != nil {
			return fmt.Errorf("failed to get cookies: %w", err)
		}

		if len(cookies) == 0 {
			fmt.Println("No cookies")
			return nil
		}

		for _, c := range cookies {
			fmt.Printf("%s=%s; Domain=%s; Path=%s\n", c.Name, c.Value, c.Domain, c.Path)
		}

		return nil
	},
}

// cookieGetCmd represents the cookie-get command
var cookieGetCmd = &cobra.Command{
	Use:   "cookie-get <name>",
	Short: "Get a cookie",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.CookieGet(cmd.GetSessionName(), args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon cookie-get failed: %s", result.Message)
			}
			if len(result.Cookies) > 0 {
				c := result.Cookies[0]
				fmt.Printf("%s=%s\n", c.Name, c.Value)
				fmt.Printf("  Domain: %s\n", c.Domain)
				fmt.Printf("  Path: %s\n", c.Path)
				fmt.Printf("  HTTPOnly: %v\n", c.HTTPOnly)
				fmt.Printf("  Secure: %v\n", c.Secure)
			}
			return nil
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		ctx := sess.Context
		if ctx == nil {
			return fmt.Errorf("no browser context")
		}

		name := args[0]
		cookies, err := ctx.Cookies()
		if err != nil {
			return fmt.Errorf("failed to get cookies: %w", err)
		}

		for _, c := range cookies {
			if c.Name == name {
				fmt.Printf("%s=%s\n", c.Name, c.Value)
				fmt.Printf("  Domain: %s\n", c.Domain)
				fmt.Printf("  Path: %s\n", c.Path)
				fmt.Printf("  HTTPOnly: %v\n", c.HttpOnly)
				fmt.Printf("  Secure: %v\n", c.Secure)
				return nil
			}
		}

		return fmt.Errorf("cookie not found: %s", name)
	},
}

// cookieSetCmd represents the cookie-set command
var cookieSetCmd = &cobra.Command{
	Use:   "cookie-set <name> <value>",
	Short: "Set a cookie",
	Args:  cobra.ExactArgs(2),
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.CookieSet(cmd.GetSessionName(), args[0], args[1], cookieDomain, "", 0, cookieHTTPOnly, cookieSecure)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon cookie-set failed: %s", result.Message)
			}
			fmt.Println(result.Message)
			return nil
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		ctx := sess.Context
		if ctx == nil {
			return fmt.Errorf("no browser context")
		}

		name := args[0]
		value := args[1]

		cookie := playwright.OptionalCookie{
			Name:  name,
			Value: value,
		}

		// Apply optional flags
		if cookieDomain != "" {
			cookie.Domain = &cookieDomain
		}

		if cookieHTTPOnly {
			cookie.HttpOnly = &cookieHTTPOnly
		}

		if cookieSecure {
			cookie.Secure = &cookieSecure
		}

		if err := ctx.AddCookies([]playwright.OptionalCookie{cookie}); err != nil {
			return fmt.Errorf("failed to set cookie: %w", err)
		}

		fmt.Printf("Cookie set: %s=%s\n", name, value)
		return nil
	},
}

// cookieDeleteCmd represents the cookie-delete command
var cookieDeleteCmd = &cobra.Command{
	Use:   "cookie-delete <name>",
	Short: "Delete a cookie",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.CookieDelete(cmd.GetSessionName(), args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon cookie-delete failed: %s", result.Message)
			}
			fmt.Println(result.Message)
			return nil
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		ctx := sess.Context
		if ctx == nil {
			return fmt.Errorf("no browser context")
		}

		name := args[0]

		if err := ctx.ClearCookies(playwright.BrowserContextClearCookiesOptions{
			Name: &name,
		}); err != nil {
			return fmt.Errorf("failed to delete cookie: %w", err)
		}

		fmt.Printf("Cookie deleted: %s\n", name)
		return nil
	},
}

// cookieClearCmd represents the cookie-clear command
var cookieClearCmd = &cobra.Command{
	Use:   "cookie-clear",
	Short: "Clear all cookies",
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.CookieClear(cmd.GetSessionName())
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon cookie-clear failed: %s", result.Message)
			}
			fmt.Println(result.Message)
			return nil
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		ctx := sess.Context
		if ctx == nil {
			return fmt.Errorf("no browser context")
		}

		if err := ctx.ClearCookies(); err != nil {
			return fmt.Errorf("failed to clear cookies: %w", err)
		}

		fmt.Println("All cookies cleared")
		return nil
	},
}

// localstorageListCmd represents the localstorage-list command
var localstorageListCmd = &cobra.Command{
	Use:   "localstorage-list",
	Short: "List localStorage",
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.LocalStorageList(cmd.GetSessionName())
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon localstorage-list failed: %s", result.Message)
			}
			if result.Storage == nil || len(result.Storage.Items) == 0 {
				fmt.Println("No localStorage items")
				return nil
			}
			for _, item := range result.Storage.Items {
				fmt.Printf("%s: %s\n", item.Key, item.Value)
			}
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

		items, err := page.Evaluate(`() => {
			let items = [];
			for (let i = 0; i < localStorage.length; i++) {
				let key = localStorage.key(i);
				items.push({ key, value: localStorage.getItem(key) });
			}
			return items;
		}`, nil)

		if err != nil {
			return fmt.Errorf("failed to list localStorage: %w", err)
		}

		if items == nil {
			fmt.Println("No localStorage items")
			return nil
		}

		fmt.Printf("localStorage items: %v\n", items)
		return nil
	},
}

// localstorageGetCmd represents the localstorage-get command
var localstorageGetCmd = &cobra.Command{
	Use:   "localstorage-get <key>",
	Short: "Get localStorage item",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.LocalStorageGet(cmd.GetSessionName(), args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon localstorage-get failed: %s", result.Message)
			}
			fmt.Println(result.Message)
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

		key := args[0]
		result, err := page.Evaluate(fmt.Sprintf(`() => localStorage.getItem('%s')`, key), nil)
		if err != nil {
			return fmt.Errorf("failed to get localStorage item: %w", err)
		}

		if result == nil {
			fmt.Printf("localStorage['%s'] not found\n", key)
		} else {
			fmt.Printf("%s\n", result)
		}

		return nil
	},
}

// localstorageSetCmd represents the localstorage-set command
var localstorageSetCmd = &cobra.Command{
	Use:   "localstorage-set <key> <value>",
	Short: "Set localStorage item",
	Args:  cobra.ExactArgs(2),
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.LocalStorageSet(cmd.GetSessionName(), args[0], args[1])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon localstorage-set failed: %s", result.Message)
			}
			fmt.Println(result.Message)
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

		key := args[0]
		value := args[1]

		_, err = page.Evaluate(fmt.Sprintf(`() => { localStorage.setItem('%s', '%s'); }`, key, value), nil)
		if err != nil {
			return fmt.Errorf("failed to set localStorage item: %w", err)
		}

		fmt.Printf("localStorage['%s'] = %s\n", key, value)
		return nil
	},
}

// localstorageDeleteCmd represents the localstorage-delete command
var localstorageDeleteCmd = &cobra.Command{
	Use:   "localstorage-delete <key>",
	Short: "Delete localStorage item",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.LocalStorageDelete(cmd.GetSessionName(), args[0])
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon localstorage-delete failed: %s", result.Message)
			}
			fmt.Println(result.Message)
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

		key := args[0]

		_, err = page.Evaluate(fmt.Sprintf(`() => { localStorage.removeItem('%s'); }`, key), nil)
		if err != nil {
			return fmt.Errorf("failed to delete localStorage item: %w", err)
		}

		fmt.Printf("localStorage['%s'] deleted\n", key)
		return nil
	},
}

// localstorageClearCmd represents the localstorage-clear command
var localstorageClearCmd = &cobra.Command{
	Use:   "localstorage-clear",
	Short: "Clear localStorage",
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err != nil {
				return err
			}
			defer client.Close()
			result, err := client.LocalStorageClear(cmd.GetSessionName())
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("daemon localstorage-clear failed: %s", result.Message)
			}
			fmt.Println(result.Message)
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

		_, err = page.Evaluate(`() => { localStorage.clear(); }`, nil)
		if err != nil {
			return fmt.Errorf("failed to clear localStorage: %w", err)
		}

		fmt.Println("localStorage cleared")
		return nil
	},
}

// sessionstorageListCmd represents the sessionstorage-list command
var sessionstorageListCmd = &cobra.Command{
	Use:   "sessionstorage-list",
	Short: "List sessionStorage",
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()
				result, err := client.SessionStorageList(cmd.GetSessionName())
				if err == nil && result.Success {
					if result.Storage == nil || len(result.Storage.Items) == 0 {
						fmt.Println("No sessionStorage items")
						return nil
					}
					for _, item := range result.Storage.Items {
						fmt.Printf("%s: %s\n", item.Key, item.Value)
					}
					return nil
				}
			}
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		items, err := page.Evaluate(`() => {
			let items = [];
			for (let i = 0; i < sessionStorage.length; i++) {
				let key = sessionStorage.key(i);
				items.push({ key, value: sessionStorage.getItem(key) });
			}
			return items;
		}`, nil)

		if err != nil {
			return fmt.Errorf("failed to list sessionStorage: %w", err)
		}

		if items == nil {
			fmt.Println("No sessionStorage items")
			return nil
		}

		fmt.Printf("sessionStorage items: %v\n", items)
		return nil
	},
}

// sessionstorageGetCmd represents the sessionstorage-get command
var sessionstorageGetCmd = &cobra.Command{
	Use:   "sessionstorage-get <key>",
	Short: "Get sessionStorage item",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()
				result, err := client.SessionStorageGet(cmd.GetSessionName(), args[0])
				if err == nil && result.Success {
					fmt.Println(result.Message)
					return nil
				}
			}
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		key := args[0]
		result, err := page.Evaluate(fmt.Sprintf(`() => sessionStorage.getItem('%s')`, key), nil)
		if err != nil {
			return fmt.Errorf("failed to get sessionStorage item: %w", err)
		}

		if result == nil {
			fmt.Printf("sessionStorage['%s'] not found\n", key)
		} else {
			fmt.Printf("%s\n", result)
		}

		return nil
	},
}

// sessionstorageSetCmd represents the sessionstorage-set command
var sessionstorageSetCmd = &cobra.Command{
	Use:   "sessionstorage-set <key> <value>",
	Short: "Set sessionStorage item",
	Args:  cobra.ExactArgs(2),
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()
				result, err := client.SessionStorageSet(cmd.GetSessionName(), args[0], args[1])
				if err == nil && result.Success {
					fmt.Println(result.Message)
					return nil
				}
			}
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		key := args[0]
		value := args[1]

		_, err = page.Evaluate(fmt.Sprintf(`() => { sessionStorage.setItem('%s', '%s'); }`, key, value), nil)
		if err != nil {
			return fmt.Errorf("failed to set sessionStorage item: %w", err)
		}

		fmt.Printf("sessionStorage['%s'] = %s\n", key, value)
		return nil
	},
}

// sessionstorageDeleteCmd represents the sessionstorage-delete command
var sessionstorageDeleteCmd = &cobra.Command{
	Use:   "sessionstorage-delete <key>",
	Short: "Delete sessionStorage item",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()
				result, err := client.SessionStorageDelete(cmd.GetSessionName(), args[0])
				if err == nil && result.Success {
					fmt.Println(result.Message)
					return nil
				}
			}
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		key := args[0]

		_, err = page.Evaluate(fmt.Sprintf(`() => { sessionStorage.removeItem('%s'); }`, key), nil)
		if err != nil {
			return fmt.Errorf("failed to delete sessionStorage item: %w", err)
		}

		fmt.Printf("sessionStorage['%s'] deleted\n", key)
		return nil
	},
}

// sessionstorageClearCmd represents the sessionstorage-clear command
var sessionstorageClearCmd = &cobra.Command{
	Use:   "sessionstorage-clear",
	Short: "Clear sessionStorage",
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()
				result, err := client.SessionStorageClear(cmd.GetSessionName())
				if err == nil && result.Success {
					fmt.Println(result.Message)
					return nil
				}
			}
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		_, err = page.Evaluate(`() => { sessionStorage.clear(); }`, nil)
		if err != nil {
			return fmt.Errorf("failed to clear sessionStorage: %w", err)
		}

		fmt.Println("sessionStorage cleared")
		return nil
	},
}

// deleteDataCmd represents the delete-data command
var deleteDataCmd = &cobra.Command{
	Use:   "delete-data",
	Short: "Delete browser data",
	RunE: func(c *cobra.Command, args []string) error {
		if daemonMode() {
			client, err := daemon.NewClient()
			if err == nil {
				defer client.Close()
				result, err := client.CookieClear(cmd.GetSessionName())
				if err == nil && result.Success {
					fmt.Println("Browser data deleted (cookies cleared)")
					return nil
				}
			}
		}

		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		ctx := sess.Context
		if ctx == nil {
			return fmt.Errorf("no browser context")
		}

		// Clear cookies
		if err := ctx.ClearCookies(); err != nil {
			return fmt.Errorf("failed to clear cookies: %w", err)
		}

		fmt.Println("Browser data deleted (cookies cleared)")
		return nil
	},
}

func init() {
	cmd.RootCmd.AddCommand(stateSaveCmd)
	cmd.RootCmd.AddCommand(stateLoadCmd)
	cmd.RootCmd.AddCommand(cookieListCmd)
	cmd.RootCmd.AddCommand(cookieGetCmd)
	cmd.RootCmd.AddCommand(cookieSetCmd)
	cmd.RootCmd.AddCommand(cookieDeleteCmd)
	cmd.RootCmd.AddCommand(cookieClearCmd)
	cmd.RootCmd.AddCommand(localstorageListCmd)
	cmd.RootCmd.AddCommand(localstorageGetCmd)
	cmd.RootCmd.AddCommand(localstorageSetCmd)
	cmd.RootCmd.AddCommand(localstorageDeleteCmd)
	cmd.RootCmd.AddCommand(localstorageClearCmd)
	cmd.RootCmd.AddCommand(sessionstorageListCmd)
	cmd.RootCmd.AddCommand(sessionstorageGetCmd)
	cmd.RootCmd.AddCommand(sessionstorageSetCmd)
	cmd.RootCmd.AddCommand(sessionstorageDeleteCmd)
	cmd.RootCmd.AddCommand(sessionstorageClearCmd)
	cmd.RootCmd.AddCommand(deleteDataCmd)

	cookieSetCmd.Flags().StringVar(&cookieDomain, "domain", "", "Cookie domain")
	cookieSetCmd.Flags().BoolVar(&cookieHTTPOnly, "httpOnly", false, "HTTPOnly flag")
	cookieSetCmd.Flags().BoolVar(&cookieSecure, "secure", false, "Secure flag")
}
