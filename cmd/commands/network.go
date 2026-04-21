package commands

import (
	"fmt"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/spf13/cobra"
)

var (
	routeStatus  int
	routeBody    string
	routeHeaders string
)

// routeCmd represents the route command
var routeCmd = &cobra.Command{
	Use:   "route <pattern>",
	Short: "Mock a network request",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		return fmt.Errorf("network routing is not implemented in the daemon CLI yet")
	},
}

// routeListCmd represents the route-list command
var routeListCmd = &cobra.Command{
	Use:   "route-list",
	Short: "List all mocked routes",
	RunE: func(c *cobra.Command, args []string) error {
		return fmt.Errorf("network routing is not implemented in the daemon CLI yet")
	},
}

// unrouteCmd represents the unroute command
var unrouteCmd = &cobra.Command{
	Use:   "unroute [pattern]",
	Short: "Remove a route or all routes",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		return fmt.Errorf("network routing is not implemented in the daemon CLI yet")
	},
}

func init() {
	cmd.RootCmd.AddCommand(routeCmd)
	cmd.RootCmd.AddCommand(routeListCmd)
	cmd.RootCmd.AddCommand(unrouteCmd)

	routeCmd.Flags().IntVar(&routeStatus, "status", 200, "Response status code")
	routeCmd.Flags().StringVar(&routeBody, "body", "", "Response body")
	routeCmd.Flags().StringVar(&routeHeaders, "header", "", "Response headers (key:value,key:value)")
}
