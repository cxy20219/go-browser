package commands

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/browserless/go-cli-browser/cmd"
	"github.com/playwright-community/playwright-go"
	"github.com/spf13/cobra"
)

var (
	consoleType  string
	codeFilename string
	tracingFile  string
	videoFile    string
)

// ConsoleMessage stores console messages
type ConsoleMessageStore struct {
	mu       sync.Mutex
	messages []ConsoleMessageEntry
}

type ConsoleMessageEntry struct {
	Type      string
	Text      string
	Location  string
	Timestamp time.Time
}

var consoleStore = &ConsoleMessageStore{}

func (s *ConsoleMessageStore) Add(msgType, text, location string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, ConsoleMessageEntry{
		Type:      msgType,
		Text:      text,
		Location:  location,
		Timestamp: time.Now(),
	})
}

func (s *ConsoleMessageStore) GetAll() []ConsoleMessageEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]ConsoleMessageEntry, len(s.messages))
	copy(result, s.messages)
	return result
}

func (s *ConsoleMessageStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}

// consoleCmd represents the console command
var consoleCmd = &cobra.Command{
	Use:   "console [type]",
	Short: "Get console messages",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		filterType := ""
		if len(args) > 0 {
			filterType = args[0]
		}

		messages := consoleStore.GetAll()
		if len(messages) == 0 {
			fmt.Println("No console messages")
			return nil
		}

		for _, msg := range messages {
			if filterType == "" || msg.Type == filterType {
				loc := msg.Location
				if loc != "" {
					fmt.Printf("[%s] %s (%s)\n", msg.Type, msg.Text, loc)
				} else {
					fmt.Printf("[%s] %s\n", msg.Type, msg.Text)
				}
			}
		}

		return nil
	},
}

// networkCmd represents the network command - show network requests/responses
var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Get network requests/responses",
	RunE: func(c *cobra.Command, args []string) error {
		return fmt.Errorf("network monitoring is not implemented yet")
	},
}

// runCodeCmd represents the run-code command
var runCodeCmd = &cobra.Command{
	Use:   "run-code [code]",
	Short: "Execute JavaScript code",
	Args: func(c *cobra.Command, args []string) error {
		if codeFilename != "" {
			if len(args) > 0 {
				return fmt.Errorf("use either code argument or --filename, not both")
			}
			return nil
		}
		return cobra.ExactArgs(1)(c, args)
	},
	RunE: func(c *cobra.Command, args []string) error {
		sess, err := cmd.GetSession()
		if err != nil {
			return err
		}

		page, err := sess.CurrentActivePage()
		if err != nil {
			return err
		}

		code := ""
		if codeFilename != "" {
			data, err := os.ReadFile(codeFilename)
			if err != nil {
				return fmt.Errorf("failed to read JavaScript file: %w", err)
			}
			code = string(data)
		} else {
			code = args[0]
		}
		result, err := page.Evaluate(code, nil)
		if err != nil {
			return err
		}

		fmt.Printf("Result: %v\n", result)
		return nil
	},
}

// tracingStartCmd represents the tracing-start command
var tracingStartCmd = &cobra.Command{
	Use:   "tracing-start [output]",
	Short: "Start tracing",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		return fmt.Errorf("tracing is not implemented yet")
	},
}

// tracingStopCmd represents the tracing-stop command
var tracingStopCmd = &cobra.Command{
	Use:   "tracing-stop",
	Short: "Stop tracing",
	RunE: func(c *cobra.Command, args []string) error {
		return fmt.Errorf("tracing is not implemented yet")
	},
}

// videoStartCmd represents the video-start command
var videoStartCmd = &cobra.Command{
	Use:   "video-start [filename]",
	Short: "Start video recording",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		return fmt.Errorf("video recording is not implemented yet")
	},
}

// videoStopCmd represents the video-stop command
var videoStopCmd = &cobra.Command{
	Use:   "video-stop",
	Short: "Stop video recording",
	RunE: func(c *cobra.Command, args []string) error {
		return fmt.Errorf("video recording is not implemented yet")
	},
}

// videoChapterCmd represents the video-chapter command
var videoChapterCmd = &cobra.Command{
	Use:   "video-chapter [name]",
	Short: "Add a chapter to video",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(c *cobra.Command, args []string) error {
		return fmt.Errorf("video recording is not implemented yet")
	},
}

func init() {
	cmd.RootCmd.AddCommand(consoleCmd)
	cmd.RootCmd.AddCommand(networkCmd)
	cmd.RootCmd.AddCommand(runCodeCmd)
	cmd.RootCmd.AddCommand(tracingStartCmd)
	cmd.RootCmd.AddCommand(tracingStopCmd)
	cmd.RootCmd.AddCommand(videoStartCmd)
	cmd.RootCmd.AddCommand(videoStopCmd)
	cmd.RootCmd.AddCommand(videoChapterCmd)

	runCodeCmd.Flags().StringVar(&codeFilename, "filename", "", "JavaScript file to run")
	tracingStartCmd.Flags().StringVar(&tracingFile, "output", "", "Tracing output file")
	videoStartCmd.Flags().StringVar(&videoFile, "filename", "", "Video filename")
}

// setupConsoleHandler sets up the console message handler for the page
func setupConsoleHandler(page playwright.Page) {
	page.OnConsole(func(msg playwright.ConsoleMessage) {
		loc := msg.Location()
		locStr := ""
		if loc != nil {
			locStr = fmt.Sprintf("%s:%d", loc.URL, loc.LineNumber)
		}
		consoleStore.Add(msg.Type(), msg.Text(), locStr)
	})
}
