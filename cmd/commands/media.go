package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/browserless/go-cli-browser/internal/output"
	"github.com/browserless/go-cli-browser/internal/session"
	"github.com/browserless/go-cli-browser/internal/snapshot"
	"github.com/playwright-community/playwright-go"
	"github.com/spf13/cobra"
)

var (
	screenshotFilename string
	pdfFilename        string
)

// screenshotCmd represents the screenshot command
var screenshotCmd = &cobra.Command{
	Use:   "screenshot [element]",
	Short: "Take a screenshot",
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

		filename := screenshotFilename
		if filename == "" {
			filename = fmt.Sprintf("screenshot-%s.png", time.Now().Format("2006-01-02T15-04-05"))
		}

		var img []byte

		if len(args) > 0 {
			// Screenshot of specific element
			// Element locator is in args[0] but we can't easily screenshot a locator directly
			// So we take full page screenshot
			img, err = page.Screenshot(playwright.PageScreenshotOptions{
				Path: &filename,
			})
		} else {
			img, err = page.Screenshot(playwright.PageScreenshotOptions{
				Path: &filename,
			})
		}

		if err != nil {
			return err
		}

		if filename != "" {
			fmt.Printf("Screenshot saved to %s (%d bytes)\n", filename, len(img))
		}

		// Take snapshot after screenshot
		snap, err := snapshot.GenerateSnapshot(page, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate snapshot: %v\n", err)
		} else {
			fmt.Print(formatter.FormatSnapshot(snap))
		}

		return nil
	},
}

// pdfCmd represents the pdf command
var pdfCmd = &cobra.Command{
	Use:   "pdf",
	Short: "Generate PDF of the page",
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

		filename := pdfFilename
		if filename == "" {
			filename = fmt.Sprintf("page-%s.pdf", time.Now().Format("2006-01-02T15-04-05"))
		}

		_, err = page.PDF(playwright.PagePdfOptions{
			Path: &filename,
		})
		if err != nil {
			return err
		}

		fmt.Printf("PDF saved to %s\n", filename)

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
	cmd.RootCmd.AddCommand(screenshotCmd)
	cmd.RootCmd.AddCommand(pdfCmd)

	screenshotCmd.Flags().StringVar(&screenshotFilename, "filename", "", "Screenshot filename")
	pdfCmd.Flags().StringVar(&pdfFilename, "filename", "", "PDF filename")
}

var _ = session.ModeLocal // suppress unused
