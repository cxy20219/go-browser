package daemon

import (
	"encoding/json"
	"time"

	"github.com/browserless/go-cli-browser/internal/snapshot"
)

// Protocol defines the IPC protocol between CLI and daemon
type Protocol struct{}

// Request represents a client request
type Request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response represents a daemon response
type Response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ResponseError  `json:"error,omitempty"`
}

// ResponseError represents an error response
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SessionParams for session operations
type SessionParams struct {
	Name string `json:"name"`
}

// OpenParams for open command
type OpenParams struct {
	SessionName string `json:"session_name,omitempty"`
	URL         string `json:"url,omitempty"`
	BrowserType string `json:"browser_type,omitempty"`
	Headless    bool   `json:"headless,omitempty"`
}

// GotoParams for goto command
type GotoParams struct {
	SessionName string `json:"session_name,omitempty"`
	URL         string `json:"url"`
}

// NavigationParams for go_back, go_forward, reload commands
type NavigationParams struct {
	SessionName string `json:"session_name,omitempty"`
}

// LocatorParams for locator-based operations.
type LocatorParams struct {
	SessionName string `json:"session_name,omitempty"`
	Locator     string `json:"locator"`
}

// FillParams for fill command.
type FillParams struct {
	SessionName string `json:"session_name,omitempty"`
	Locator     string `json:"locator"`
	Text        string `json:"text"`
	Submit      bool   `json:"submit,omitempty"`
}

// EvalParams for eval command.
type EvalParams struct {
	SessionName string `json:"session_name,omitempty"`
	Expression  string `json:"expression"`
}

// ResizeParams for resize command.
type ResizeParams struct {
	SessionName string `json:"session_name,omitempty"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// KeyboardTextParams for typing text.
type KeyboardTextParams struct {
	SessionName string `json:"session_name,omitempty"`
	Text        string `json:"text"`
}

// KeyboardKeyParams for key operations.
type KeyboardKeyParams struct {
	SessionName string `json:"session_name,omitempty"`
	Key         string `json:"key"`
}

// MouseMoveParams for moving the mouse.
type MouseMoveParams struct {
	SessionName string  `json:"session_name,omitempty"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
}

// MouseButtonParams for mouse button operations.
type MouseButtonParams struct {
	SessionName string `json:"session_name,omitempty"`
	Button      string `json:"button,omitempty"`
}

// MouseWheelParams for scrolling the mouse wheel.
type MouseWheelParams struct {
	SessionName string  `json:"session_name,omitempty"`
	DeltaX      float64 `json:"delta_x"`
	DeltaY      float64 `json:"delta_y"`
}

// TabNewParams for creating a new tab.
type TabNewParams struct {
	SessionName string `json:"session_name,omitempty"`
	URL         string `json:"url,omitempty"`
}

// TabIndexParams for tab operations that target an index.
type TabIndexParams struct {
	SessionName string `json:"session_name,omitempty"`
	Index       int    `json:"index"`
}

// SelectParams for select command.
type SelectParams struct {
	SessionName string `json:"session_name,omitempty"`
	Locator     string `json:"locator"`
	Value       string `json:"value"`
}

// DragParams for drag command.
type DragParams struct {
	SessionName   string `json:"session_name,omitempty"`
	SourceLocator string `json:"source_locator"`
	TargetLocator string `json:"target_locator"`
}

// UploadParams for upload command.
type UploadParams struct {
	SessionName string `json:"session_name,omitempty"`
	Locator     string `json:"locator,omitempty"`
	FilePath    string `json:"file_path"`
}

// CheckParams for check command.
type CheckParams struct {
	SessionName string `json:"session_name,omitempty"`
	Locator     string `json:"locator"`
}

// UncheckParams for uncheck command.
type UncheckParams struct {
	SessionName string `json:"session_name,omitempty"`
	Locator     string `json:"locator"`
}

// Result represents a generic result
type Result struct {
	Success    bool            `json:"success"`
	Message    string          `json:"message,omitempty"`
	Value      interface{}     `json:"value,omitempty"`
	Session    *SessionInfo    `json:"session,omitempty"`
	Sessions   []string        `json:"sessions,omitempty"`
	Tabs       []TabInfo       `json:"tabs,omitempty"`
	Snapshot   *SnapshotInfo   `json:"snapshot,omitempty"`
	Screenshot *ScreenshotInfo `json:"screenshot,omitempty"`
}

// SessionInfo information about a session
type SessionInfo struct {
	Name        string `json:"name"`
	Mode        string `json:"mode"`
	PageCount   int    `json:"page_count"`
	CurrentURL  string `json:"current_url,omitempty"`
	BrowserType string `json:"browser_type"`
	PID         int    `json:"pid"`
	CDPPort     int    `json:"cdp_port"`
}

// SnapshotInfo page snapshot info
type SnapshotInfo struct {
	URL       string                `json:"url"`
	Title     string                `json:"title"`
	Timestamp time.Time             `json:"timestamp"`
	Elements  []snapshot.ElementRef `json:"elements"`
}

// TabInfo information about a browser tab.
type TabInfo struct {
	Index   int    `json:"index"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Current bool   `json:"current"`
}

// ElementRef element reference
type ElementRef struct {
	Ref   string            `json:"ref"`
	Type  string            `json:"type"`
	Text  string            `json:"text,omitempty"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

// ScreenshotInfo screenshot result
type ScreenshotInfo struct {
	Path string `json:"path"`
	Size int    `json:"size"`
}

// DaemonInfo daemon status
type DaemonInfo struct {
	PID        int      `json:"pid"`
	SocketPath string   `json:"socket_path"`
	Sessions   []string `json:"sessions"`
	Version    string   `json:"version"`
}

// Method names
const (
	MethodPing         = "ping"
	MethodStatus       = "status"
	MethodStop         = "stop"
	MethodOpen         = "open"
	MethodGoto         = "goto"
	MethodGoBack       = "go_back"
	MethodGoForward    = "go_forward"
	MethodReload       = "reload"
	MethodClose        = "close"
	MethodSnapshot     = "snapshot"
	MethodScreenshot   = "screenshot"
	MethodClick        = "click"
	MethodFill         = "fill"
	MethodHover        = "hover"
	MethodEval         = "eval"
	MethodResize       = "resize"
	MethodType         = "type"
	MethodPress        = "press"
	MethodKeyDown      = "keydown"
	MethodKeyUp        = "keyup"
	MethodMouseMove    = "mousemove"
	MethodMouseDown    = "mousedown"
	MethodMouseUp      = "mouseup"
	MethodMouseWheel   = "mousewheel"
	MethodTabList      = "tab_list"
	MethodTabNew       = "tab_new"
	MethodTabClose     = "tab_close"
	MethodTabSelect    = "tab_select"
	MethodListSessions = "list_sessions"
	MethodSelect       = "select"
	MethodDrag         = "drag"
	MethodUpload       = "upload"
	MethodCheck        = "check"
	MethodUncheck      = "uncheck"
	MethodDblClick     = "dblclick"
)
