package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/browserless/go-cli-browser/internal/browser"
	"github.com/browserless/go-cli-browser/internal/session"
	pagesnapshot "github.com/browserless/go-cli-browser/internal/snapshot"
	"github.com/playwright-community/playwright-go"
)

const (
	// DefaultPort is the default TCP port on Windows
	DefaultPort = 9223
	// SocketName is the Unix socket name on Unix systems
	SocketName = "go-browser.sock"
	// ProtocolVersion for compatibility
	ProtocolVersion = "1.0"
)

// Server is the daemon server
type Server struct {
	listener   net.Listener
	sessions   *session.SessionManager
	playwright *playwright.Playwright
	browsers   map[string]*BrowserHandle
	mu         sync.RWMutex
	socketPath string
	pid        int
	stopCh     chan struct{}
	stopping   bool
	headless   bool // true = headless, false = headed
}

var globalServer *Server

// BrowserHandle holds browser instance and its info
type BrowserHandle struct {
	Name        string
	Browser     playwright.Browser
	Context     playwright.BrowserContext
	CDPPort     int
	PID         int
	Opts        *session.SessionOptions
	Refs        *pagesnapshot.RefCache
	CurrentPage int
}

// GetServer returns the global server instance
func GetServer() *Server {
	return globalServer
}

// GetStopCh returns the server's stop channel
func (s *Server) GetStopCh() chan struct{} {
	return s.stopCh
}

// GetSocketPath returns the socket path for IPC
func GetSocketPath() string {
	if IsWindows() {
		return fmt.Sprintf("localhost:%d", DefaultPort)
	}
	return filepath.Join(os.TempDir(), SocketName)
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return os.PathSeparator == '\\'
}

// NewServer creates a new daemon server
func NewServer(headless bool) (*Server, error) {
	socketPath := GetSocketPath()

	// Initialize Playwright
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run playwright: %w", err)
	}

	// Initialize session manager
	session.Init()

	s := &Server{
		sessions:   session.GetManager(),
		playwright: pw,
		browsers:   make(map[string]*BrowserHandle),
		socketPath: socketPath,
		pid:        os.Getpid(),
		stopCh:     make(chan struct{}),
		headless:   headless,
	}

	globalServer = s
	return s, nil
}

// Start starts the daemon server
func (s *Server) Start() error {
	// Remove existing socket file on Unix
	if !IsWindows() {
		os.Remove(s.socketPath)
	}

	// Listen on socket
	var err error
	if IsWindows() {
		s.listener, err = net.Listen("tcp", s.socketPath)
	} else {
		s.listener, err = net.Listen("unix", s.socketPath)
	}
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.socketPath, err)
	}

	// Set socket permissions on Unix
	if !IsWindows() {
		os.Chmod(s.socketPath, 0600)
	}

	// Save daemon info
	s.saveDaemonInfo()

	go s.acceptLoop()
	return nil
}

// Stop stops the daemon server
func (s *Server) Stop() error {
	s.mu.Lock()
	if s.stopping {
		s.mu.Unlock()
		return nil
	}
	s.stopping = true
	s.mu.Unlock()

	close(s.stopCh)

	// Wait a bit for acceptLoop to exit and pending requests to complete
	time.Sleep(500 * time.Millisecond)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Close all browsers
	for name, handle := range s.browsers {
		if handle.Context != nil {
			handle.Context.Close()
		}
		if handle.Browser != nil {
			handle.Browser.Close()
		}
		delete(s.browsers, name)
	}

	// Stop playwright
	if s.playwright != nil {
		s.playwright.Stop()
	}

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Remove socket file
	if !IsWindows() {
		os.Remove(s.socketPath)
	}

	// Remove daemon info
	s.removeDaemonInfo()

	globalServer = nil
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.RLock()
			stopping := s.stopping
			s.mu.RUnlock()
			if stopping {
				return
			}
			select {
			case <-s.stopCh:
				return
			default:
				continue
			}
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		// Check if server is stopping
		s.mu.RLock()
		stopping := s.stopping
		s.mu.RUnlock()

		// Use Read deadline to avoid blocking forever
		if stopping {
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		}

		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err != io.EOF {
				// Log error but don't close connection on decode error
			}
			return
		}

		resp := s.handleRequest(&req)
		if err := encoder.Encode(resp); err != nil {
			return
		}
	}
}

func (s *Server) handleRequest(req *Request) *Response {
	var result interface{}
	var err error

	switch req.Method {
	case MethodPing:
		result = s.handlePing()
	case MethodStatus:
		result = s.handleStatus()
	case MethodStop:
		result, err = s.handleStop()
	case MethodOpen:
		result, err = s.handleOpen(req.Params)
	case MethodGoto:
		result, err = s.handleGoto(req.Params)
	case MethodGoBack:
		result, err = s.handleGoBack(req.Params)
	case MethodGoForward:
		result, err = s.handleGoForward(req.Params)
	case MethodReload:
		result, err = s.handleReload(req.Params)
	case MethodClose:
		result, err = s.handleClose(req.Params)
	case MethodSnapshot:
		result, err = s.handleSnapshot(req.Params)
	case MethodScreenshot:
		result, err = s.handleScreenshot(req.Params)
	case MethodPdf:
		result, err = s.handlePdf(req.Params)
	case MethodClick:
		result, err = s.handleClick(req.Params)
	case MethodFill:
		result, err = s.handleFill(req.Params)
	case MethodHover:
		result, err = s.handleHover(req.Params)
	case MethodEval:
		result, err = s.handleEval(req.Params)
	case MethodResize:
		result, err = s.handleResize(req.Params)
	case MethodType:
		result, err = s.handleType(req.Params)
	case MethodPress:
		result, err = s.handlePress(req.Params)
	case MethodKeyDown:
		result, err = s.handleKeyDown(req.Params)
	case MethodKeyUp:
		result, err = s.handleKeyUp(req.Params)
	case MethodMouseMove:
		result, err = s.handleMouseMove(req.Params)
	case MethodMouseDown:
		result, err = s.handleMouseDown(req.Params)
	case MethodMouseUp:
		result, err = s.handleMouseUp(req.Params)
	case MethodMouseWheel:
		result, err = s.handleMouseWheel(req.Params)
	case MethodTabList:
		result, err = s.handleTabList(req.Params)
	case MethodTabNew:
		result, err = s.handleTabNew(req.Params)
	case MethodTabClose:
		result, err = s.handleTabClose(req.Params)
	case MethodTabSelect:
		result, err = s.handleTabSelect(req.Params)
	case MethodDblClick:
		result, err = s.handleDblClick(req.Params)
	case MethodSelect:
		result, err = s.handleSelect(req.Params)
	case MethodCheck:
		result, err = s.handleCheck(req.Params)
	case MethodUncheck:
		result, err = s.handleUncheck(req.Params)
	case MethodDrag:
		result, err = s.handleDrag(req.Params)
	case MethodUpload:
		result, err = s.handleUpload(req.Params)
	case MethodStateSave:
		result, err = s.handleStateSave(req.Params)
	case MethodStateLoad:
		result, err = s.handleStateLoad(req.Params)
	case MethodDialogAccept:
		result, err = s.handleDialogAccept(req.Params)
	case MethodDialogDismiss:
		result, err = s.handleDialogDismiss(req.Params)
	case MethodCookieList:
		result, err = s.handleCookieList(req.Params)
	case MethodCookieGet:
		result, err = s.handleCookieGet(req.Params)
	case MethodCookieSet:
		result, err = s.handleCookieSet(req.Params)
	case MethodCookieDelete:
		result, err = s.handleCookieDelete(req.Params)
	case MethodCookieClear:
		result, err = s.handleCookieClear(req.Params)
	case MethodLocalStorage:
		result, err = s.handleLocalStorage(req.Params)
	case MethodSessionStorage:
		result, err = s.handleSessionStorage(req.Params)
	case MethodCloseAll:
		result, err = s.handleCloseAll()
	case MethodListSessions:
		result = s.handleListSessions()
	default:
		return &Response{
			ID: req.ID,
			Error: &ResponseError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}

	if err != nil {
		return &Response{
			ID: req.ID,
			Error: &ResponseError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}

	resultJSON, _ := json.Marshal(result)
	return &Response{
		ID:     req.ID,
		Result: resultJSON,
	}
}

func (s *Server) handlePing() *DaemonInfo {
	return &DaemonInfo{
		PID:        s.pid,
		SocketPath: s.socketPath,
		Version:    ProtocolVersion,
	}
}

func (s *Server) handleStatus() *DaemonInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]string, 0, len(s.browsers))
	for name := range s.browsers {
		sessions = append(sessions, name)
	}

	return &DaemonInfo{
		PID:        s.pid,
		SocketPath: s.socketPath,
		Sessions:   sessions,
		Version:    ProtocolVersion,
	}
}

func (s *Server) handleOpen(paramsJSON json.RawMessage) (*Result, error) {
	var params OpenParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	name := params.SessionName
	if name == "" {
		name = "default"
	}

	// Check if session already exists
	s.mu.RLock()
	handle, exists := s.browsers[name]
	s.mu.RUnlock()

	if exists && handle.Context != nil {
		// Reconnect to existing browser
		page, err := handle.Context.NewPage()
		if err != nil {
			return nil, fmt.Errorf("failed to create page: %w", err)
		}
		if params.URL != "" {
			if _, err := page.Goto(params.URL, playwright.PageGotoOptions{
				Timeout: floatPtr(30000),
			}); err != nil {
				return nil, fmt.Errorf("navigation failed: %w", err)
			}
		}
		handle.CurrentPage = len(handle.Context.Pages()) - 1
		return &Result{
			Success: true,
			Message: fmt.Sprintf("Reconnected to session %s", name),
			Session: &SessionInfo{
				Name:        name,
				Mode:        string(session.ModeLocal),
				PageCount:   len(handle.Context.Pages()),
				CurrentURL:  page.URL(),
				BrowserType: handle.Opts.Browser,
				CDPPort:     handle.CDPPort,
				PID:         handle.PID,
			},
		}, nil
	}

	// Launch new browser
	browserType := params.BrowserType
	if browserType == "" {
		browserType = "chromium"
	}

	// Use daemon's headless setting - the daemon's mode (set at startup via --headed)
	// is authoritative for all browser launches in this daemon
	headless := s.headless

	browserImpl := browser.NewLocalBrowser()
	result, err := browserImpl.Launch(&session.SessionOptions{
		Browser:  browserType,
		Headless: headless,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	ctx := result.Context

	// Create page
	page, err := ctx.NewPage()
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	// Navigate if URL provided
	if params.URL != "" {
		if _, err := page.Goto(params.URL, playwright.PageGotoOptions{
			Timeout: floatPtr(30000),
		}); err != nil {
			// Non-fatal, continue anyway
		}
	}

	// Save session
	sess := s.sessions.GetOrCreate(name)
	sess.Context = ctx
	sess.Mode = session.ModeLocal
	sess.BrowserType = browserType
	sess.Headless = params.Headless
	sess.AddPage(page)
	s.sessions.Set(name, sess)

	// Store browser handle
	handle = &BrowserHandle{
		Name:        name,
		Browser:     result.Browser,
		Context:     ctx,
		CDPPort:     result.CDPPort,
		PID:         result.Pid,
		Opts:        &session.SessionOptions{Browser: browserType, Headless: params.Headless},
		Refs:        pagesnapshot.NewRefCache(),
		CurrentPage: 0,
	}

	s.mu.Lock()
	s.browsers[name] = handle
	s.mu.Unlock()

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Opened session %s", name),
		Session: &SessionInfo{
			Name:        name,
			Mode:        "local",
			PageCount:   len(ctx.Pages()),
			CurrentURL:  page.URL(),
			BrowserType: browserType,
			CDPPort:     result.CDPPort,
			PID:         result.Pid,
		},
	}, nil
}

func (s *Server) handleGoto(paramsJSON json.RawMessage) (*Result, error) {
	var params GotoParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if _, err := page.Goto(params.URL, playwright.PageGotoOptions{
		Timeout: floatPtr(30000),
	}); err != nil {
		return nil, fmt.Errorf("navigation failed: %w", err)
	}

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Navigated to %s", params.URL),
	}, nil
}

func (s *Server) handleGoBack(paramsJSON json.RawMessage) (*Result, error) {
	var params NavigationParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if _, err := page.GoBack(); err != nil {
		// Non-fatal, might not have history
	}

	return &Result{
		Success: true,
		Message: "Navigated back",
	}, nil
}

func (s *Server) handleGoForward(paramsJSON json.RawMessage) (*Result, error) {
	var params NavigationParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if _, err := page.GoForward(); err != nil {
		// Non-fatal, might not have history
	}

	return &Result{
		Success: true,
		Message: "Navigated forward",
	}, nil
}

func (s *Server) handleReload(paramsJSON json.RawMessage) (*Result, error) {
	var params NavigationParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if _, err := page.Reload(); err != nil {
		return nil, fmt.Errorf("reload failed: %w", err)
	}

	return &Result{
		Success: true,
		Message: "Page reloaded",
	}, nil
}

func (s *Server) handleClose(paramsJSON json.RawMessage) (*Result, error) {
	var params SessionParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	name := params.Name
	if name == "" {
		name = "default"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	handle, exists := s.browsers[name]
	if !exists {
		return &Result{
			Success: true,
			Message: fmt.Sprintf("Session %s already closed", name),
		}, nil
	}

	if handle.Context != nil {
		handle.Context.Close()
	}
	if handle.Browser != nil {
		handle.Browser.Close()
	}

	delete(s.browsers, name)
	s.sessions.Delete(name)

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Closed session %s", name),
	}, nil
}

func (s *Server) handleSnapshot(paramsJSON json.RawMessage) (*Result, error) {
	var params SessionParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, handle, err := s.pageAndHandleForSession(params.Name)
	if err != nil {
		return nil, err
	}
	snap, err := pagesnapshot.GenerateSnapshot(page, 3)
	if err != nil {
		return nil, fmt.Errorf("snapshot failed: %w", err)
	}
	if handle.Refs == nil {
		handle.Refs = pagesnapshot.NewRefCache()
	}
	handle.Refs.BuildFromSnapshot(snap)

	return &Result{
		Success: true,
		Session: &SessionInfo{
			Name:        handle.Name,
			CurrentURL:  snap.URL,
			BrowserType: handle.Opts.Browser,
		},
		Snapshot: &SnapshotInfo{
			URL:       snap.URL,
			Title:     snap.Title,
			Timestamp: snap.Timestamp,
			Elements:  snap.Elements,
		},
	}, nil
}

func (s *Server) handleScreenshot(paramsJSON json.RawMessage) (*Result, error) {
	var params SessionParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.Name)
	if err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("screenshot-%s.png", time.Now().Format("2006-01-02T15-04-05"))
	img, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path: &filename,
	})
	if err != nil {
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Screenshot saved to %s", filename),
		Screenshot: &ScreenshotInfo{
			Path: filename,
			Size: len(img),
		},
	}, nil
}

func (s *Server) handlePdf(paramsJSON json.RawMessage) (*Result, error) {
	var params SessionParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.Name)
	if err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("page-%s.pdf", time.Now().Format("2006-01-02T15-04-05"))
	_, err = page.PDF(playwright.PagePdfOptions{
		Path: &filename,
	})
	if err != nil {
		return nil, fmt.Errorf("pdf failed: %w", err)
	}

	return &Result{
		Success: true,
		Message: fmt.Sprintf("PDF saved to %s", filename),
		Pdf: &PdfInfo{
			Path: filename,
		},
	}, nil
}

func (s *Server) handleClick(paramsJSON json.RawMessage) (*Result, error) {
	var params LocatorParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	loc, err := s.resolveLocator(page, handle, params.Locator)
	if err != nil {
		return nil, err
	}
	if err := loc.Click(playwright.LocatorClickOptions{
		Timeout: floatPtr(30000),
	}); err != nil {
		return nil, fmt.Errorf("click failed: %w", err)
	}
	return &Result{Success: true, Message: "Clicked"}, nil
}

func (s *Server) handleFill(paramsJSON json.RawMessage) (*Result, error) {
	var params FillParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	loc, err := s.resolveLocator(page, handle, params.Locator)
	if err != nil {
		return nil, err
	}
	if err := loc.Fill(params.Text); err != nil {
		return nil, fmt.Errorf("fill failed: %w", err)
	}
	if params.Submit {
		if err := loc.Press("Enter"); err != nil {
			return nil, fmt.Errorf("submit failed: %w", err)
		}
	}
	return &Result{Success: true, Message: "Filled"}, nil
}

func (s *Server) handleHover(paramsJSON json.RawMessage) (*Result, error) {
	var params LocatorParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	loc, err := s.resolveLocator(page, handle, params.Locator)
	if err != nil {
		return nil, err
	}
	if err := loc.Hover(playwright.LocatorHoverOptions{
		Timeout: floatPtr(30000),
	}); err != nil {
		return nil, fmt.Errorf("hover failed: %w", err)
	}
	return &Result{Success: true, Message: "Hovered"}, nil
}

func (s *Server) handleEval(paramsJSON json.RawMessage) (*Result, error) {
	var params EvalParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	value, err := page.Evaluate(params.Expression, nil)
	if err != nil {
		return nil, fmt.Errorf("eval failed: %w", err)
	}
	return &Result{Success: true, Value: value}, nil
}

func (s *Server) handleResize(paramsJSON json.RawMessage) (*Result, error) {
	var params ResizeParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if err := page.SetViewportSize(params.Width, params.Height); err != nil {
		return nil, fmt.Errorf("resize failed: %w", err)
	}
	return &Result{Success: true, Message: "Viewport resized"}, nil
}

func (s *Server) handleType(paramsJSON json.RawMessage) (*Result, error) {
	var params KeyboardTextParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if err := page.Keyboard().Type(params.Text, playwright.KeyboardTypeOptions{}); err != nil {
		return nil, fmt.Errorf("type failed: %w", err)
	}
	return &Result{Success: true, Message: "Typed"}, nil
}

func (s *Server) handlePress(paramsJSON json.RawMessage) (*Result, error) {
	var params KeyboardKeyParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if err := page.Keyboard().Press(params.Key, playwright.KeyboardPressOptions{}); err != nil {
		return nil, fmt.Errorf("press failed: %w", err)
	}
	return &Result{Success: true, Message: "Pressed"}, nil
}

func (s *Server) handleKeyDown(paramsJSON json.RawMessage) (*Result, error) {
	var params KeyboardKeyParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if err := page.Keyboard().Down(params.Key); err != nil {
		return nil, fmt.Errorf("keydown failed: %w", err)
	}
	return &Result{Success: true, Message: "Key down"}, nil
}

func (s *Server) handleKeyUp(paramsJSON json.RawMessage) (*Result, error) {
	var params KeyboardKeyParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if err := page.Keyboard().Up(params.Key); err != nil {
		return nil, fmt.Errorf("keyup failed: %w", err)
	}
	return &Result{Success: true, Message: "Key up"}, nil
}

func (s *Server) handleMouseMove(paramsJSON json.RawMessage) (*Result, error) {
	var params MouseMoveParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if err := page.Mouse().Move(params.X, params.Y); err != nil {
		return nil, fmt.Errorf("mousemove failed: %w", err)
	}
	return &Result{Success: true, Message: "Mouse moved"}, nil
}

func (s *Server) handleMouseDown(paramsJSON json.RawMessage) (*Result, error) {
	var params MouseButtonParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	options, err := mouseButtonOptions(params.Button)
	if err != nil {
		return nil, err
	}
	if err := page.Mouse().Down(options); err != nil {
		return nil, fmt.Errorf("mousedown failed: %w", err)
	}
	return &Result{Success: true, Message: "Mouse down"}, nil
}

func (s *Server) handleMouseUp(paramsJSON json.RawMessage) (*Result, error) {
	var params MouseButtonParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	options, err := mouseButtonUpOptions(params.Button)
	if err != nil {
		return nil, err
	}
	if err := page.Mouse().Up(options); err != nil {
		return nil, fmt.Errorf("mouseup failed: %w", err)
	}
	return &Result{Success: true, Message: "Mouse up"}, nil
}

func (s *Server) handleMouseWheel(paramsJSON json.RawMessage) (*Result, error) {
	var params MouseWheelParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, err := s.pageForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	if err := page.Mouse().Wheel(params.DeltaX, params.DeltaY); err != nil {
		return nil, fmt.Errorf("mousewheel failed: %w", err)
	}
	return &Result{Success: true, Message: "Mouse wheel scrolled"}, nil
}

func (s *Server) handleTabList(paramsJSON json.RawMessage) (*Result, error) {
	var params SessionParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.Name)
	if err != nil {
		return nil, err
	}

	tabs := tabInfos(handle)
	return &Result{
		Success: true,
		Message: fmt.Sprintf("%d tabs", len(tabs)),
		Tabs:    tabs,
		Session: &SessionInfo{
			Name:      handle.Name,
			PageCount: len(tabs),
		},
	}, nil
}

func (s *Server) handleTabNew(paramsJSON json.RawMessage) (*Result, error) {
	var params TabNewParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	page, err := handle.Context.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create tab: %w", err)
	}
	if params.URL != "" {
		if _, err := page.Goto(params.URL, playwright.PageGotoOptions{
			Timeout: floatPtr(30000),
		}); err != nil {
			return nil, fmt.Errorf("navigation failed: %w", err)
		}
	}

	pages := handle.Context.Pages()
	handle.CurrentPage = len(pages) - 1
	return &Result{
		Success: true,
		Message: "Opened new tab",
		Session: &SessionInfo{
			Name:       handle.Name,
			PageCount:  len(pages),
			CurrentURL: page.URL(),
		},
		Tabs: tabInfos(handle),
	}, nil
}

func (s *Server) handleTabClose(paramsJSON json.RawMessage) (*Result, error) {
	var params TabIndexParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	pages := handle.Context.Pages()
	index := params.Index
	if index < 0 {
		index = handle.CurrentPage
	}
	if index < 0 || index >= len(pages) {
		return nil, fmt.Errorf("invalid tab index %d", index)
	}

	if err := pages[index].Close(); err != nil {
		return nil, fmt.Errorf("failed to close tab %d: %w", index, err)
	}

	pages = handle.Context.Pages()
	if len(pages) == 0 {
		handle.CurrentPage = 0
		return &Result{
			Success: true,
			Message: "All tabs closed",
			Session: &SessionInfo{Name: handle.Name, PageCount: 0},
		}, nil
	}

	if handle.CurrentPage >= len(pages) {
		handle.CurrentPage = len(pages) - 1
	} else if index < handle.CurrentPage {
		handle.CurrentPage--
	}
	if err := pages[handle.CurrentPage].BringToFront(); err != nil {
		return nil, fmt.Errorf("failed to activate tab %d: %w", handle.CurrentPage, err)
	}

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Closed tab %d", index),
		Session: &SessionInfo{
			Name:       handle.Name,
			PageCount:  len(pages),
			CurrentURL: pages[handle.CurrentPage].URL(),
		},
		Tabs: tabInfos(handle),
	}, nil
}

func (s *Server) handleTabSelect(paramsJSON json.RawMessage) (*Result, error) {
	var params TabIndexParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	pages := handle.Context.Pages()
	if params.Index < 0 || params.Index >= len(pages) {
		return nil, fmt.Errorf("invalid tab index %d", params.Index)
	}
	handle.CurrentPage = params.Index
	if err := pages[handle.CurrentPage].BringToFront(); err != nil {
		return nil, fmt.Errorf("failed to activate tab %d: %w", handle.CurrentPage, err)
	}

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Selected tab %d", handle.CurrentPage),
		Session: &SessionInfo{
			Name:       handle.Name,
			PageCount:  len(pages),
			CurrentURL: pages[handle.CurrentPage].URL(),
		},
		Tabs: tabInfos(handle),
	}, nil
}

func (s *Server) handleDblClick(paramsJSON json.RawMessage) (*Result, error) {
	var params LocatorParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	_, selector, err := s.resolveLocatorToSelector(page, handle, params.Locator)
	if err != nil {
		return nil, err
	}
	timeout := floatPtr(30000)
	if err := page.Dblclick(selector, playwright.PageDblclickOptions{
		Timeout: timeout,
	}); err != nil {
		return nil, fmt.Errorf("dblclick failed: %w", err)
	}
	return &Result{Success: true, Message: "Double-clicked"}, nil
}

func (s *Server) resolveLocatorToSelector(page playwright.Page, handle *BrowserHandle, locatorStr string) (playwright.Locator, string, error) {
	trimmed := strings.TrimSpace(locatorStr)
	if !pagesnapshot.IsRef(trimmed) {
		return page.Locator(locatorStr), locatorStr, nil
	}

	if handle != nil && handle.Refs != nil {
		if selector, ok := handle.Refs.Selector(trimmed); ok {
			return page.Locator(selector), selector, nil
		}
	}

	selector, err := pagesnapshot.ResolveRefToSelector(page, trimmed)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve ref %s: %w", trimmed, err)
	}
	if selector == "" {
		return nil, "", fmt.Errorf("ref %s not found; run snapshot before using snapshot refs", trimmed)
	}
	return page.Locator(selector), selector, nil
}

func (s *Server) handleSelect(paramsJSON json.RawMessage) (*Result, error) {
	var params SelectParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	loc, err := s.resolveLocator(page, handle, params.Locator)
	if err != nil {
		return nil, err
	}
	values := []string{params.Value}
	_, err = loc.SelectOption(playwright.SelectOptionValues{Values: &values}, playwright.LocatorSelectOptionOptions{})
	if err != nil {
		return nil, fmt.Errorf("select failed: %w", err)
	}
	return &Result{Success: true, Message: "Option selected"}, nil
}

func (s *Server) handleCheck(paramsJSON json.RawMessage) (*Result, error) {
	var params CheckParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	loc, err := s.resolveLocator(page, handle, params.Locator)
	if err != nil {
		return nil, err
	}
	if err := loc.Check(playwright.LocatorCheckOptions{
		Timeout: floatPtr(30000),
	}); err != nil {
		return nil, fmt.Errorf("check failed: %w", err)
	}
	return &Result{Success: true, Message: "Checked"}, nil
}

func (s *Server) handleUncheck(paramsJSON json.RawMessage) (*Result, error) {
	var params UncheckParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	loc, err := s.resolveLocator(page, handle, params.Locator)
	if err != nil {
		return nil, err
	}
	if err := loc.Uncheck(playwright.LocatorUncheckOptions{
		Timeout: floatPtr(30000),
	}); err != nil {
		return nil, fmt.Errorf("uncheck failed: %w", err)
	}
	return &Result{Success: true, Message: "Unchecked"}, nil
}

func (s *Server) handleDrag(paramsJSON json.RawMessage) (*Result, error) {
	var params DragParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}
	sourceLoc, err := s.resolveLocator(page, handle, params.SourceLocator)
	if err != nil {
		return nil, err
	}
	targetLoc, err := s.resolveLocator(page, handle, params.TargetLocator)
	if err != nil {
		return nil, err
	}
	if err := sourceLoc.DragTo(targetLoc, playwright.LocatorDragToOptions{}); err != nil {
		return nil, fmt.Errorf("drag failed: %w", err)
	}
	return &Result{Success: true, Message: "Dragged"}, nil
}

func (s *Server) handleUpload(paramsJSON json.RawMessage) (*Result, error) {
	var params UploadParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, _, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	locatorStr := params.Locator
	if locatorStr == "" {
		locatorStr = "input[type=file]"
	}
	loc := page.Locator(locatorStr)
	if err := loc.SetInputFiles(params.FilePath); err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	return &Result{Success: true, Message: "Uploaded"}, nil
}

func (s *Server) handleStateSave(paramsJSON json.RawMessage) (*Result, error) {
	var params StateParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	if handle.Context == nil {
		return nil, fmt.Errorf("no browser context")
	}

	filename := params.Filename
	if filename == "" {
		filename = fmt.Sprintf("storage-state-%d.json", os.Getpid())
	}

	state, err := handle.Context.StorageState()
	if err != nil {
		return nil, fmt.Errorf("failed to get storage state: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write state file: %w", err)
	}

	return &Result{Success: true, Message: fmt.Sprintf("State saved to %s", filename)}, nil
}

func (s *Server) handleStateLoad(paramsJSON json.RawMessage) (*Result, error) {
	var params StateParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	if handle.Context == nil {
		return nil, fmt.Errorf("no browser context")
	}

	filename := params.Filename
	if filename == "" {
		return nil, fmt.Errorf("filename required")
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state playwright.StorageState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

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

	if err := handle.Context.AddCookies(cookies); err != nil {
		return nil, fmt.Errorf("failed to add cookies: %w", err)
	}

	return &Result{Success: true, Message: fmt.Sprintf("State loaded from %s", filename)}, nil
}

func (s *Server) handleDialogAccept(paramsJSON json.RawMessage) (*Result, error) {
	var params DialogParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, _, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	promptText := params.PromptText
	if promptText != "" {
		page.OnDialog(func(dialog playwright.Dialog) {
			dialog.Accept(promptText)
		})
	} else {
		page.OnDialog(func(dialog playwright.Dialog) {
			dialog.Accept()
		})
	}

	return &Result{Success: true, Message: "Dialog accepted"}, nil
}

func (s *Server) handleDialogDismiss(paramsJSON json.RawMessage) (*Result, error) {
	var params DialogParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, _, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Dismiss()
	})

	return &Result{Success: true, Message: "Dialog dismissed"}, nil
}

func (s *Server) handleCookieList(paramsJSON json.RawMessage) (*Result, error) {
	var params SessionParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.Name)
	if err != nil {
		return nil, err
	}

	if handle.Context == nil {
		return nil, fmt.Errorf("no browser context")
	}

	cookies, err := handle.Context.Cookies()
	if err != nil {
		return nil, fmt.Errorf("failed to get cookies: %w", err)
	}

	cookieInfos := make([]CookieInfo, len(cookies))
	for i, c := range cookies {
		cookieInfos[i] = CookieInfo{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  int64(c.Expires),
			HTTPOnly: c.HttpOnly,
			Secure:   c.Secure,
		}
	}

	return &Result{Success: true, Message: fmt.Sprintf("%d cookies", len(cookies)), Cookies: cookieInfos}, nil
}

func (s *Server) handleCookieGet(paramsJSON json.RawMessage) (*Result, error) {
	var params CookieParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	if handle.Context == nil {
		return nil, fmt.Errorf("no browser context")
	}

	cookies, err := handle.Context.Cookies()
	if err != nil {
		return nil, fmt.Errorf("failed to get cookies: %w", err)
	}

	for _, c := range cookies {
		if c.Name == params.Name {
			return &Result{
				Success: true,
				Value:   c.Value,
				Message: fmt.Sprintf("%s=%s", c.Name, c.Value),
				Cookies: []CookieInfo{{
					Name:     c.Name,
					Value:    c.Value,
					Domain:   c.Domain,
					Path:     c.Path,
					Expires:  int64(c.Expires),
					HTTPOnly: c.HttpOnly,
					Secure:   c.Secure,
				}},
			}, nil
		}
	}

	return nil, fmt.Errorf("cookie not found: %s", params.Name)
}

func (s *Server) handleCookieSet(paramsJSON json.RawMessage) (*Result, error) {
	var params CookieParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	if handle.Context == nil {
		return nil, fmt.Errorf("no browser context")
	}

	cookie := playwright.OptionalCookie{
		Name:  params.Name,
		Value: params.Value,
	}

	if params.Domain != "" {
		cookie.Domain = &params.Domain
	}
	if params.Path != "" {
		cookie.Path = &params.Path
	}
	if params.Expires != 0 {
		exp := float64(params.Expires)
		cookie.Expires = &exp
	}
	if params.HTTPOnly {
		cookie.HttpOnly = &params.HTTPOnly
	}
	if params.Secure {
		cookie.Secure = &params.Secure
	}

	if err := handle.Context.AddCookies([]playwright.OptionalCookie{cookie}); err != nil {
		return nil, fmt.Errorf("failed to set cookie: %w", err)
	}

	return &Result{Success: true, Message: fmt.Sprintf("Cookie set: %s=%s", params.Name, params.Value)}, nil
}

func (s *Server) handleCookieDelete(paramsJSON json.RawMessage) (*Result, error) {
	var params CookieParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	if handle.Context == nil {
		return nil, fmt.Errorf("no browser context")
	}

	if err := handle.Context.ClearCookies(playwright.BrowserContextClearCookiesOptions{
		Name: &params.Name,
	}); err != nil {
		return nil, fmt.Errorf("failed to delete cookie: %w", err)
	}

	return &Result{Success: true, Message: fmt.Sprintf("Cookie deleted: %s", params.Name)}, nil
}

func (s *Server) handleCookieClear(paramsJSON json.RawMessage) (*Result, error) {
	var params SessionParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	_, handle, err := s.pageAndHandleForSession(params.Name)
	if err != nil {
		return nil, err
	}

	if handle.Context == nil {
		return nil, fmt.Errorf("no browser context")
	}

	if err := handle.Context.ClearCookies(); err != nil {
		return nil, fmt.Errorf("failed to clear cookies: %w", err)
	}

	return &Result{Success: true, Message: "All cookies cleared"}, nil
}

func (s *Server) handleLocalStorage(paramsJSON json.RawMessage) (*Result, error) {
	var params LocalStorageParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, _, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	switch params.Action {
	case "list":
		items, err := page.Evaluate(`() => {
			let items = [];
			for (let i = 0; i < localStorage.length; i++) {
				let key = localStorage.key(i);
				items.push({ key, value: localStorage.getItem(key) });
			}
			return items;
		}`, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list localStorage: %w", err)
		}
		if items == nil {
			return &Result{Success: true, Message: "No localStorage items", Storage: &StorageInfo{}}, nil
		}
		if itemList, ok := items.([]interface{}); ok {
			storageItems := make([]StorageItem, len(itemList))
			for i, item := range itemList {
				if m, ok := item.(map[string]interface{}); ok {
					if key, ok := m["key"].(string); ok {
						storageItems[i].Key = key
					}
					if value, ok := m["value"].(string); ok {
						storageItems[i].Value = value
					}
				}
			}
			return &Result{Success: true, Message: fmt.Sprintf("localStorage items: %d", len(storageItems)), Storage: &StorageInfo{Items: storageItems}}, nil
		}
		return &Result{Success: true, Message: fmt.Sprintf("localStorage: %v", items)}, nil

	case "get":
		if params.Key == "" {
			return nil, fmt.Errorf("key required")
		}
		result, err := page.Evaluate(fmt.Sprintf(`() => localStorage.getItem('%s')`, params.Key), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get localStorage item: %w", err)
		}
		if result == nil {
			return &Result{Success: true, Message: fmt.Sprintf("localStorage['%s'] not found", params.Key)}, nil
		}
		return &Result{Success: true, Value: result, Message: fmt.Sprintf("%v", result)}, nil

	case "set":
		if params.Key == "" {
			return nil, fmt.Errorf("key required")
		}
		_, err := page.Evaluate(fmt.Sprintf(`() => { localStorage.setItem('%s', '%s'); }`, params.Key, params.Value), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to set localStorage item: %w", err)
		}
		return &Result{Success: true, Message: fmt.Sprintf("localStorage['%s'] = %s", params.Key, params.Value)}, nil

	case "delete":
		if params.Key == "" {
			return nil, fmt.Errorf("key required")
		}
		_, err := page.Evaluate(fmt.Sprintf(`() => { localStorage.removeItem('%s'); }`, params.Key), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to delete localStorage item: %w", err)
		}
		return &Result{Success: true, Message: fmt.Sprintf("localStorage['%s'] deleted", params.Key)}, nil

	case "clear":
		_, err := page.Evaluate(`() => { localStorage.clear(); }`, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to clear localStorage: %w", err)
		}
		return &Result{Success: true, Message: "localStorage cleared"}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s", params.Action)
	}
}

func (s *Server) handleSessionStorage(paramsJSON json.RawMessage) (*Result, error) {
	var params SessionStorageParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, err
	}

	page, _, err := s.pageAndHandleForSession(params.SessionName)
	if err != nil {
		return nil, err
	}

	switch params.Action {
	case "list":
		items, err := page.Evaluate(`() => {
			let items = [];
			for (let i = 0; i < sessionStorage.length; i++) {
				let key = sessionStorage.key(i);
				items.push({ key, value: sessionStorage.getItem(key) });
			}
			return items;
		}`, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list sessionStorage: %w", err)
		}
		if items == nil {
			return &Result{Success: true, Message: "No sessionStorage items", Storage: &StorageInfo{}}, nil
		}
		if itemList, ok := items.([]interface{}); ok {
			storageItems := make([]StorageItem, len(itemList))
			for i, item := range itemList {
				if m, ok := item.(map[string]interface{}); ok {
					if key, ok := m["key"].(string); ok {
						storageItems[i].Key = key
					}
					if value, ok := m["value"].(string); ok {
						storageItems[i].Value = value
					}
				}
			}
			return &Result{Success: true, Message: fmt.Sprintf("sessionStorage items: %d", len(storageItems)), Storage: &StorageInfo{Items: storageItems}}, nil
		}
		return &Result{Success: true, Message: fmt.Sprintf("sessionStorage: %v", items)}, nil

	case "get":
		if params.Key == "" {
			return nil, fmt.Errorf("key required")
		}
		result, err := page.Evaluate(fmt.Sprintf(`() => sessionStorage.getItem('%s')`, params.Key), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get sessionStorage item: %w", err)
		}
		if result == nil {
			return &Result{Success: true, Message: fmt.Sprintf("sessionStorage['%s'] not found", params.Key)}, nil
		}
		return &Result{Success: true, Value: result, Message: fmt.Sprintf("%v", result)}, nil

	case "set":
		if params.Key == "" {
			return nil, fmt.Errorf("key required")
		}
		_, err := page.Evaluate(fmt.Sprintf(`() => { sessionStorage.setItem('%s', '%s'); }`, params.Key, params.Value), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to set sessionStorage item: %w", err)
		}
		return &Result{Success: true, Message: fmt.Sprintf("sessionStorage['%s'] = %s", params.Key, params.Value)}, nil

	case "delete":
		if params.Key == "" {
			return nil, fmt.Errorf("key required")
		}
		_, err := page.Evaluate(fmt.Sprintf(`() => { sessionStorage.removeItem('%s'); }`, params.Key), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to delete sessionStorage item: %w", err)
		}
		return &Result{Success: true, Message: fmt.Sprintf("sessionStorage['%s'] deleted", params.Key)}, nil

	case "clear":
		_, err := page.Evaluate(`() => { sessionStorage.clear(); }`, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to clear sessionStorage: %w", err)
		}
		return &Result{Success: true, Message: "sessionStorage cleared"}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s", params.Action)
	}
}

func (s *Server) handleListSessions() *Result {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]string, 0, len(s.browsers))
	for name := range s.browsers {
		sessions = append(sessions, name)
	}

	return &Result{
		Success:  true,
		Message:  fmt.Sprintf("%d sessions", len(sessions)),
		Sessions: sessions,
	}
}

func (s *Server) handleCloseAll() (*Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for name, handle := range s.browsers {
		if handle.Context != nil {
			handle.Context.Close()
		}
		if handle.Browser != nil {
			handle.Browser.Close()
		}
		delete(s.browsers, name)
		s.sessions.Delete(name)
		count++
	}

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Closed %d sessions", count),
	}, nil
}

func tabInfos(handle *BrowserHandle) []TabInfo {
	pages := handle.Context.Pages()
	tabs := make([]TabInfo, 0, len(pages))
	for i, page := range pages {
		title, _ := page.Title()
		tabs = append(tabs, TabInfo{
			Index:   i,
			URL:     page.URL(),
			Title:   title,
			Current: i == handle.CurrentPage,
		})
	}
	return tabs
}

func mouseButtonOptions(button string) (playwright.MouseDownOptions, error) {
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

func mouseButtonUpOptions(button string) (playwright.MouseUpOptions, error) {
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

func (s *Server) pageForSession(name string) (playwright.Page, error) {
	page, _, err := s.pageAndHandleForSession(name)
	return page, err
}

func (s *Server) pageAndHandleForSession(name string) (playwright.Page, *BrowserHandle, error) {
	if name == "" {
		name = "default"
	}

	s.mu.RLock()
	handle, exists := s.browsers[name]
	s.mu.RUnlock()

	if !exists || handle.Context == nil {
		return nil, nil, fmt.Errorf("session %s not found", name)
	}

	pages := handle.Context.Pages()
	if len(pages) == 0 {
		return nil, nil, fmt.Errorf("no pages in session %s", name)
	}
	if handle.CurrentPage < 0 || handle.CurrentPage >= len(pages) {
		handle.CurrentPage = 0
	}
	return pages[handle.CurrentPage], handle, nil
}

func (s *Server) resolveLocator(page playwright.Page, handle *BrowserHandle, locatorStr string) (playwright.Locator, error) {
	trimmed := strings.TrimSpace(locatorStr)
	if !pagesnapshot.IsRef(trimmed) {
		return page.Locator(locatorStr), nil
	}

	if handle != nil && handle.Refs != nil {
		if selector, ok := handle.Refs.Selector(trimmed); ok {
			return page.Locator(selector), nil
		}
	}

	selector, err := pagesnapshot.ResolveRefToSelector(page, trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ref %s: %w", trimmed, err)
	}
	if selector == "" {
		return nil, fmt.Errorf("ref %s not found; run snapshot before using snapshot refs", trimmed)
	}
	return page.Locator(selector), nil
}

func (s *Server) handleStop() (*Result, error) {
	// Return success immediately, then trigger async shutdown
	go func() {
		time.Sleep(100 * time.Millisecond)
		if globalServer != nil {
			globalServer.Stop()
		}
	}()

	return &Result{
		Success: true,
		Message: "Daemon stopped",
	}, nil
}

func (s *Server) saveDaemonInfo() {
	info := &DaemonInfo{
		PID:        s.pid,
		SocketPath: s.socketPath,
		Version:    ProtocolVersion,
	}

	data, _ := json.Marshal(info)
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".go-cli-browser", "daemon.json")
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, data, 0600)
}

func (s *Server) removeDaemonInfo() {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".go-cli-browser", "daemon.json")
	os.Remove(path)
}

func floatPtr(v int) *float64 {
	f := float64(v)
	return &f
}
