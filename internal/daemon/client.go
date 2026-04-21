package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	// ConnectTimeout for daemon connection
	ConnectTimeout = 5 * time.Second
	// RequestTimeout for each request
	RequestTimeout = 30 * time.Second
)

// Client is the CLI client that connects to the daemon
type Client struct {
	socketPath string
	conn       net.Conn
}

// NewClient creates a new daemon client
func NewClient() (*Client, error) {
	socketPath := GetSocketPath()
	return &Client{socketPath: socketPath}, nil
}

// Connect connects to the daemon
func (c *Client) Connect() error {
	var err error

	if IsWindows() {
		c.conn, err = net.DialTimeout("tcp", c.socketPath, ConnectTimeout)
	} else {
		c.conn, err = net.DialTimeout("unix", c.socketPath, ConnectTimeout)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	return nil
}

// Close closes the connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Send sends a request and returns the response
func (c *Client) Send(method string, params interface{}) (*Response, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	if err := c.conn.SetDeadline(time.Now().Add(RequestTimeout)); err != nil {
		return nil, fmt.Errorf("failed to set request timeout: %w", err)
	}
	defer c.conn.SetDeadline(time.Time{})

	// Marshal params
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req := &Request{
		ID:     fmt.Sprintf("%d", time.Now().UnixNano()),
		Method: method,
		Params: paramsJSON,
	}

	// Send request
	encoder := json.NewEncoder(c.conn)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Receive response
	decoder := json.NewDecoder(c.conn)
	resp := &Response{}
	if err := decoder.Decode(resp); err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	return resp, nil
}

// Ping checks if daemon is running
func (c *Client) Ping() (*DaemonInfo, error) {
	resp, err := c.Send(MethodPing, nil)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	var info DaemonInfo
	if err := json.Unmarshal(resp.Result, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// Status returns daemon status
func (c *Client) Status() (*DaemonInfo, error) {
	resp, err := c.Send(MethodStatus, nil)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	var info DaemonInfo
	if err := json.Unmarshal(resp.Result, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// Open opens a browser session
func (c *Client) Open(name, url, browserType string, headless bool) (*Result, error) {
	params := OpenParams{
		SessionName: name,
		URL:         url,
		BrowserType: browserType,
		Headless:    headless,
	}

	resp, err := c.Send(MethodOpen, params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	var result Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Goto navigates to a URL
func (c *Client) Goto(name, url string) (*Result, error) {
	params := GotoParams{
		SessionName: name,
		URL:         url,
	}

	resp, err := c.Send(MethodGoto, params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	var result Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GoBack navigates back
func (c *Client) GoBack(name string) (*Result, error) {
	params := NavigationParams{SessionName: name}
	resp, err := c.Send(MethodGoBack, params)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}
	var result Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GoForward navigates forward
func (c *Client) GoForward(name string) (*Result, error) {
	params := NavigationParams{SessionName: name}
	resp, err := c.Send(MethodGoForward, params)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}
	var result Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Reload reloads the page
func (c *Client) Reload(name string) (*Result, error) {
	params := NavigationParams{SessionName: name}
	resp, err := c.Send(MethodReload, params)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}
	var result Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CloseSession closes a session
func (c *Client) CloseSession(name string) (*Result, error) {
	params := SessionParams{Name: name}

	resp, err := c.Send(MethodClose, params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	var result Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListSessions lists all sessions
func (c *Client) ListSessions() (*Result, error) {
	resp, err := c.Send(MethodListSessions, nil)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	var result Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Snapshot gets a snapshot of the current page
func (c *Client) Snapshot(name string) (*Result, error) {
	params := SessionParams{Name: name}
	resp, err := c.Send(MethodSnapshot, params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	var result Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Click clicks a locator in a daemon session.
func (c *Client) Click(name, locator string) (*Result, error) {
	return c.sendResult(MethodClick, LocatorParams{SessionName: name, Locator: locator})
}

// Fill fills a locator in a daemon session.
func (c *Client) Fill(name, locator, text string, submit bool) (*Result, error) {
	return c.sendResult(MethodFill, FillParams{
		SessionName: name,
		Locator:     locator,
		Text:        text,
		Submit:      submit,
	})
}

// Hover hovers a locator in a daemon session.
func (c *Client) Hover(name, locator string) (*Result, error) {
	return c.sendResult(MethodHover, LocatorParams{SessionName: name, Locator: locator})
}

// Eval evaluates JavaScript in a daemon session.
func (c *Client) Eval(name, expression string) (*Result, error) {
	return c.sendResult(MethodEval, EvalParams{SessionName: name, Expression: expression})
}

// Resize sets the viewport size in a daemon session.
func (c *Client) Resize(name string, width, height int) (*Result, error) {
	return c.sendResult(MethodResize, ResizeParams{SessionName: name, Width: width, Height: height})
}

// Type types text into the focused element in a daemon session.
func (c *Client) Type(name, text string) (*Result, error) {
	return c.sendResult(MethodType, KeyboardTextParams{SessionName: name, Text: text})
}

// Press presses a keyboard key in a daemon session.
func (c *Client) Press(name, key string) (*Result, error) {
	return c.sendResult(MethodPress, KeyboardKeyParams{SessionName: name, Key: key})
}

// KeyDown presses and holds a keyboard key in a daemon session.
func (c *Client) KeyDown(name, key string) (*Result, error) {
	return c.sendResult(MethodKeyDown, KeyboardKeyParams{SessionName: name, Key: key})
}

// KeyUp releases a keyboard key in a daemon session.
func (c *Client) KeyUp(name, key string) (*Result, error) {
	return c.sendResult(MethodKeyUp, KeyboardKeyParams{SessionName: name, Key: key})
}

// MouseMove moves the mouse in a daemon session.
func (c *Client) MouseMove(name string, x, y float64) (*Result, error) {
	return c.sendResult(MethodMouseMove, MouseMoveParams{SessionName: name, X: x, Y: y})
}

// MouseDown presses a mouse button in a daemon session.
func (c *Client) MouseDown(name, button string) (*Result, error) {
	return c.sendResult(MethodMouseDown, MouseButtonParams{SessionName: name, Button: button})
}

// MouseUp releases a mouse button in a daemon session.
func (c *Client) MouseUp(name, button string) (*Result, error) {
	return c.sendResult(MethodMouseUp, MouseButtonParams{SessionName: name, Button: button})
}

// MouseWheel scrolls the mouse wheel in a daemon session.
func (c *Client) MouseWheel(name string, deltaX, deltaY float64) (*Result, error) {
	return c.sendResult(MethodMouseWheel, MouseWheelParams{SessionName: name, DeltaX: deltaX, DeltaY: deltaY})
}

// TabList lists tabs in a daemon session.
func (c *Client) TabList(name string) (*Result, error) {
	return c.sendResult(MethodTabList, SessionParams{Name: name})
}

// TabNew creates a new tab in a daemon session.
func (c *Client) TabNew(name, url string) (*Result, error) {
	return c.sendResult(MethodTabNew, TabNewParams{SessionName: name, URL: url})
}

// TabClose closes a tab by index in a daemon session. Use -1 for the current tab.
func (c *Client) TabClose(name string, index int) (*Result, error) {
	return c.sendResult(MethodTabClose, TabIndexParams{SessionName: name, Index: index})
}

// TabSelect selects a tab by index in a daemon session.
func (c *Client) TabSelect(name string, index int) (*Result, error) {
	return c.sendResult(MethodTabSelect, TabIndexParams{SessionName: name, Index: index})
}

// DblClick double-clicks a locator in a daemon session.
func (c *Client) DblClick(name, locator string) (*Result, error) {
	return c.sendResult(MethodDblClick, LocatorParams{SessionName: name, Locator: locator})
}

// Select selects an option in a dropdown in a daemon session.
func (c *Client) Select(name, locator, value string) (*Result, error) {
	return c.sendResult(MethodSelect, SelectParams{SessionName: name, Locator: locator, Value: value})
}

// Check checks a checkbox or radio in a daemon session.
func (c *Client) Check(name, locator string) (*Result, error) {
	return c.sendResult(MethodCheck, CheckParams{SessionName: name, Locator: locator})
}

// Uncheck unchecks a checkbox in a daemon session.
func (c *Client) Uncheck(name, locator string) (*Result, error) {
	return c.sendResult(MethodUncheck, UncheckParams{SessionName: name, Locator: locator})
}

// Drag drags a source element to a target in a daemon session.
func (c *Client) Drag(name, sourceLocator, targetLocator string) (*Result, error) {
	return c.sendResult(MethodDrag, DragParams{SessionName: name, SourceLocator: sourceLocator, TargetLocator: targetLocator})
}

// Upload uploads a file in a daemon session.
func (c *Client) Upload(name, locator, filePath string) (*Result, error) {
	return c.sendResult(MethodUpload, UploadParams{SessionName: name, Locator: locator, FilePath: filePath})
}

// Screenshot takes a screenshot in a daemon session.
func (c *Client) Screenshot(name string) (*Result, error) {
	return c.sendResult(MethodScreenshot, SessionParams{Name: name})
}

// Stop stops the daemon server
func (c *Client) Stop() (*Result, error) {
	return c.sendResult(MethodStop, nil)
}

func (c *Client) sendResult(method string, params interface{}) (*Result, error) {
	resp, err := c.Send(method, params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	var result Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// IsDaemonRunning checks if the daemon is running
func IsDaemonRunning() bool {
	socketPath := GetSocketPath()

	var conn net.Conn
	var err error

	if IsWindows() {
		conn, err = net.DialTimeout("tcp", socketPath, 2*time.Second)
	} else {
		conn, err = net.DialTimeout("unix", socketPath, 2*time.Second)
	}

	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetDaemonInfo reads daemon info from disk
func GetDaemonInfo() (*DaemonInfo, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".go-cli-browser", "daemon.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("daemon not running")
	}

	var info DaemonInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}
