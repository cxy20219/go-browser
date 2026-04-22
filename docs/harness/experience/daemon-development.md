# 守护进程开发经验

## 1. CLI 架构核心限制

### 问题描述
go-cli-browser 是 CLI 工具，每次命令执行都是独立进程。浏览器作为子进程启动，在命令执行完毕后随着进程退出而关闭。

### 根因
- Go 的 `os.Executable()` + `exec.Command()` 只能在当前进程内 fork，无法真正让子进程脱离父进程存活
- 即使用了 `CREATE_NO_WINDOW` 和 `cmd.Start()`，子进程仍会随父进程退出而被终止
- Playwright 的 Browser 对象在 Go 中无法直接获取底层浏览器进程的 PID

### 解决方案
实现守护进程模式：
- 主进程 fork 后立即退出，守护进程独立运行
- CLI 命令通过 IPC (TCP socket 或 Unix socket) 与守护进程通信
- 守护进程持有 BrowserContext 和所有会话状态

### 关键代码
```go
// 守护进程启动时监听 socket
if IsWindows() {
    s.listener, err = net.Listen("tcp", s.socketPath)
} else {
    s.listener, err = net.Listen("unix", s.socketPath)
}

// CLI 命令通过 client 发送请求
client, _ := daemon.NewClient()
result, _ := client.Open("default", url, "chromium", true)
```

---

## 2. Windows 不支持 Unix Domain Socket

### 问题描述
在 Linux/macOS 上使用 Unix socket (`/tmp/go-browser.sock`)，但在 Windows 上必须使用 TCP socket。

### 解决方案
自动检测操作系统：
```go
func IsWindows() bool {
    return os.PathSeparator == '\\'
}

func GetSocketPath() string {
    if IsWindows() {
        return "localhost:9223"  // TCP on Windows
    }
    return filepath.Join(os.TempDir(), "go-browser.sock")  // Unix socket
}
```

### 注意事项
- Windows 上 TCP port 9233 可能被占用，需要重试机制
- Unix socket 使用后需要 `os.Chmod(socketPath, 0600)` 设置权限

---

## 3. Playwright Browser PID 获取问题

### 问题描述
`LaunchResult.Pid` 存储的是 CLI 进程 PID，而非实际浏览器进程 PID：
```go
return &LaunchResult{
    Pid: os.Getpid(),  // Bug: 这是 CLI 的 PID，不是浏览器 PID
}
```

### 根因
Playwright 的 Browser 接口没有暴露底层进程 PID。

### 影响
- 无法通过 PID 检测浏览器是否崩溃
- 自愈机制无法准确判断浏览器进程状态

### 临时方案
- 通过 `Browser.IsConnected()` 检测连接状态
- 通过 CDP 端口检测浏览器是否响应
- 定期发送 ping 命令检测浏览器健康状态

---

## 4. Cobra 命令 Flag 传递问题

### 问题描述
在 `start` 命令中使用：
```go
daemonArgs := []string{"daemon"}
if daemonHeaded {
    daemonArgs = append(daemonArgs, "--headed")
}
```

但 `go-browser daemon --headed` 会报错 `unknown flag: --headed`。

### 根因
`--headed` flag 只在 `startCmd` 中定义，`daemonCmd` 没有这个 flag。

### 解决方案
使用环境变量或配置文件传递选项：
```go
// 更好的方案：通过环境变量
os.Setenv("GO_BROWSER_HEADED", "true")
cmd := exec.Command(exePath, "daemon")
```

或者在启动 daemon 前写入配置文件：
```go
// 写入配置
home, _ := os.UserHomeDir()
configPath := filepath.Join(home, ".go-cli-browser", "daemon.conf")
os.WriteFile(configPath, []byte("headed=true"), 0644)

cmd := exec.Command(exePath, "daemon")
```

---

## 5. JSON-RPC 协议设计

### 简洁协议定义
```go
type Request struct {
    ID     string          `json:"id"`
    Method string          `json:"method"`
    Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    ID     string          `json:"id"`
    Result json.RawMessage `json:"result,omitempty"`
    Error  *ResponseError  `json:"error,omitempty"`
}
```

### 路由实现
```go
func (s *Server) handleRequest(req *Request) *Response {
    switch req.Method {
    case MethodPing:
        return s.handlePing()
    case MethodOpen:
        result, err := s.handleOpen(req.Params)
        // ...
    }
}
```

---

## 6. Playwright 常见错误处理

### page.URL() 返回值数量
```go
// 错误：page.URL() 只返回一个值
url, _ := page.URL()

// 正确：page.URL() 只返回一个值
url := page.URL()
```

### BrowserContext.Pages() 返回值
```go
// 正确：Pages() 返回 []Page，一个值
pages := ctx.Pages()
```

---

## 7. 守护进程无法正常停止

### 问题现象
执行 `go-browser stop` 后，输出 "Daemon stopped" 但进程仍在运行，端口仍被占用。

### 根因分析
`handleStop()` 函数中调用 `close(s.stopCh)` 导致：
1. `acceptLoop()` 中的 `listener.Accept()` 立即返回错误
2. 连接被关闭（因为 handleConnection defer 了 `conn.Close()`）
3. 响应 encoder 还未来得及发送数据，连接就已关闭
4. daemonCmd 中的 `<-sigCh` 继续阻塞，无法退出

### 解决方案
```go
func (s *Server) handleStop() (*Result, error) {
    // 异步关闭，避免阻塞响应发送
    go func() {
        time.Sleep(100 * time.Millisecond)
        if globalServer != nil {
            globalServer.Stop()
        }
    }()
    return &Result{Success: true, Message: "Daemon stopped"}, nil
}
```

### 关键教训
守护进程的停止流程需要考虑：先发送响应，再关闭连接，最后清理资源。

---

## 8. 守护进程启动后立即退出

### 问题现象
执行 `go-browser start` 后输出 "Warning: Daemon may not have started properly"，但端口实际已被监听。

### 根因分析
Playwright 初始化（下载浏览器等）需要较长时间，而 `start` 命令只等待 500ms 就检测 daemon 是否响应。

### 解决方案
增加启动等待时间并使用轮询：
```go
// 等待 Playwright 初始化完成
for i := 0; i < 30; i++ {
    time.Sleep(200 * time.Millisecond)
    if daemon.IsDaemonRunning() {
        fmt.Println("Daemon is ready")
        return nil
    }
}
```

### 关键教训
对于需要初始化外部依赖（浏览器、驱动等）的服务，不能简单用固定延迟，必须轮询检测就绪状态。

---

## 9. CLI 命令绕过守护进程架构

### 问题现象
执行 `go-browser open` 后，`go-browser status` 显示 Sessions: 0。

### 根因分析
`openCmd` 在检测到无持久化会话时，直接调用 `browserImpl.Launch()` 启动本地浏览器，完全绕过了 daemon 的 `s.browsers` map。

### 解决方案
在命令执行前检查 daemon 是否运行，优先使用 IPC：
```go
if daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
    client, err := daemon.NewClient()
    if err == nil {
        result, err := client.Open(sessionName, url, opts.Browser, !opts.Headless)
        // 处理结果
        return nil
    }
}
// 回退到本地模式
```

### 关键教训
守护进程架构下，所有命令必须优先通过 IPC 与 daemon 通信，只有在 daemon 不可用时才回退到本地模式。

---

## 10. Windows TCP 连接检测超时

### 问题现象
`IsDaemonRunning()` 在 Windows 上返回 false，但 daemon 实际正在运行。

### 根因分析
TCP 连接超时设置过短（500ms），在 Windows 网络环境下可能不够。

### 解决方案
```go
func IsDaemonRunning() bool {
    if IsWindows() {
        conn, err = net.DialTimeout("tcp", socketPath, 2*time.Second)  // 增加到 2s
    } else {
        conn, err = net.DialTimeout("unix", socketPath, 500*time.Millisecond)
    }
    // ...
}
```

### 关键教训
Windows 网络栈与 Unix 不同，同样操作可能需要更长超时时间。

---

## 11. 异步关闭时的竞态条件

### 问题现象
快速连续执行 `stop` 和 `status` 时，status 可能仍显示进程运行。

### 根因分析
`handleStop` 异步调用 `server.Stop()`，但 `server.Stop()` 中有 500ms 延迟才真正清理资源。

### 解决方案
`server.Stop()` 设计为幂等操作：
```go
func (s *Server) Stop() error {
    s.mu.Lock()
    if s.stopping {  // 防止重复关闭
        s.mu.Unlock()
        return nil
    }
    s.stopping = true
    s.mu.Unlock()
    // 清理逻辑...
}
```

### 关键教训
异步关闭模式下，清理函数必须是幂等的，能处理多次调用。

---

## 12. daemonCmd 的信号响应

### 问题现象
守护进程接收到 IPC stop 请求后，进程不退出。

### 根因分析
`daemonCmd` 只等待系统信号（SIGINT/SIGTERM），不监听 IPC 层面的停止请求。

### 解决方案
让 daemonCmd 监听 server 的 stopCh：
```go
select {
case <-sigCh:
case <-server.GetStopCh():
}
server.Stop()
```

### 关键教训
守护进程需要同时响应多种停止信号：系统信号、IPC 命令、自身停止 channel。

---

## 13. 守护进程优雅关闭设计

### 核心设计
守护进程需要处理 SIGINT (Ctrl+C) 和 SIGTERM 信号，同时响应 IPC 停止请求：

```go
var daemonCmd = &cobra.Command{
    Use:   "daemon",
    Hidden: true,
    RunE: func(c *cobra.Command, args []string) error {
        server, _ := daemon.NewServer()
        server.Start()

        // 等待信号
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

        select {
        case <-sigCh:
            // 系统信号
        case <-server.GetStopCh():
            // IPC 停止请求
        }

        server.Stop()
        return nil
    },
}
```

### 关键点
- 使用缓冲 channel 避免信号丢失
- 先通知再关闭，确保清理完成
- 关闭时需要关闭所有浏览器连接
- stopCh 关闭是异步的，避免响应卡死

---

## 14. 进程间通信检测

### 检测守护进程是否运行
```go
func IsDaemonRunning() bool {
    if IsWindows() {
        conn, err := net.DialTimeout("tcp", "localhost:9223", 2*time.Second)
        if err != nil { return false }
        conn.Close()
        return true
    }
    // Unix socket
    conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
    if err != nil { return false }
    conn.Close()
    return true
}
```

### 检测浏览器是否存活
```go
// 通过 IsConnected 检测
if !browser.IsConnected() {
    // 浏览器已断开，尝试重连或重启
}

// 通过 CDP 端口检测
func CheckCDPEndpoint(port int) bool {
    url := fmt.Sprintf("http://localhost:%d/json", port)
    resp, err := http.Get(url)
    if err != nil { return false }
    resp.Body.Close()
    return resp.StatusCode == 200
}
```

### 关键教训
- Windows 上 TCP 超时需要更长（2s）
- 使用 `IsConnected()` 而非 PID 判断浏览器状态
- 定期 ping/pong 检测连接健康

---

## 15. 自愈机制设计

### 检测周期
```go
func (s *Server) healthCheckLoop() {
    ticker := time.NewTicker(30 * time.Second)
    for {
        <-ticker.C
        for name, handle := range s.browsers {
            if !handle.Browser.IsConnected() {
                s.healSession(name)
            }
        }
    }
}
```

### 恢复流程
```go
func (s *Server) healSession(name string) error {
    // 1. 关闭旧连接
    if handle.Context != nil {
        handle.Context.Close()
    }

    // 2. 重新启动浏览器
    result, err := browser.NewLocalBrowser().Launch(handle.Opts)
    if err != nil { return err }

    // 3. 更新句柄
    handle.Browser = result.Browser
    handle.Context = result.Context
    handle.PID = result.PID

    return nil
}
```

---

## 16. 常见调试命令

### Windows 查看端口占用
```bash
netstat -ano | findstr 9223
```

### Windows 强制终止进程
```bash
taskkill //F //IM go-browser.exe
```

### 查看进程
```bash
# Windows
tasklist | findstr "go-browser"
```

### 检查 socket 文件（Linux）
```bash
ls -la /tmp/go-browser.sock
```

### 检查 daemon 是否响应
```bash
# TCP 测试
curl http://localhost:9223 2>/dev/null || echo "Not responding"

# 或者使用 telnet
telnet localhost 9223
```

### 添加服务端调试日志
在 `internal/daemon/server.go` 中：
```go
fmt.Printf("[DEBUG] handleOpen: session=%s, URL=%s\n", name, params.URL)
```

### 重建并测试
```bash
go build -o go-browser.exe .
go-browser.exe stop
go-browser.exe start
go-browser.exe open <url> --headless=false
go-browser.exe snapshot
```

### 检查 Daemon 状态
```bash
go-browser.exe list
# 输出: * Session: default (status=running)
```

---

## 17. 测试页面环境问题

### 问题描述
在 daemon 模式下使用 `open` 命令打开测试页面时，`snapshot` 返回的 Elements 为空。

### 根因分析
原测试页面 `test/index.html` 使用 Vue 3 + ES modules：
```html
<script type="module" src="./app.js"></script>
```

Python SimpleHTTP 服务器不支持 ES modules 的 `import/export` 语法，导致：
- 浏览器能加载 HTML 结构
- 但 Vue 组件无法初始化
- 页面实际内容为空

### 排查思路
1. 使用 `--raw` flag 检查 open 命令返回的原始数据
2. 对比 daemon 模式和本地模式的输出差异
3. 检查 HTTP 服务器是否正确服务 JS 模块

### 解决方案
创建不依赖构建工具的简单测试页面（如 `test/simple.html`），或使用支持 ES modules 的 HTTP 服务器（如 `npx serve`）。

### 验证方法
```bash
# 检查 HTTP 服务器是否支持 ES modules
curl -I http://localhost:8080/app.js
# 看返回的 Content-Type 是否为 application/javascript
```

---

## 18. 命令行引号导致 Locator 解析失败

### 问题描述
`click` 命令在 daemon 模式下超时（30秒），但相同页面在本地模式正常工作。

### 根因分析
在 Windows cmd 中执行命令时：
```bash
# 错误写法 - 引号被包含在参数中
go-browser.exe click \"#btn-submit\"
# 实际传递的 locator 字符串是: "#btn-submit" (包含引号)

# 正确写法
go-browser.exe click #btn-submit
```

带引号的 locator 字符串无法匹配任何 CSS 选择器，Playwright 等待元素出现直到超时。

### 排查思路
1. 添加服务端调试日志打印接收到的参数
2. 对比命令行传递的参数与预期值的差异
3. 检查错误信息中的具体失败原因

### 解决方案
使用 `--raw` 或检查实际传递的参数。CSS 选择器不需要引号包裹。

### 验证方法
```bash
# 在 server.go 中添加调试日志
fmt.Printf("[DEBUG] params.Locator=%q\n", params.Locator)
# 如果输出是 "\"#btn-submit\"" 说明引号被包含进去了
```

---

## 19. Daemon 连接 I/O Timeout

### 问题描述
执行 `click` 等命令时报错：`failed to receive response: read tcp 127.0.0.1:xxxx->127.0.0.1:9223: i/o timeout`

### 常见原因
1. **Playwright 操作阻塞**：locator 找不到元素或元素不可交互，Playwright 等待直到超时
2. **Daemon 进程卡死**：浏览器操作导致 Daemon 主线程阻塞
3. **网络问题**：TCP 连接异常中断

### 排查思路
1. 先用 `snapshot` 命令确认页面状态正常
2. 用 `eval` 执行简单 JS 确认页面可交互
3. 使用 `--raw` 查看命令的详细错误信息
4. 添加调试日志定位阻塞点

### 解决方案
- 确认 locator 正确且元素可见
- 检查浏览器是否有弹窗阻挡
- 确认页面已完全加载

---

## 20. Daemon 与本地模式的行为差异

### 关键区别

| 特性 | Daemon 模式 | 本地模式 |
|------|------------|---------|
| Session 持久性 | 跨命令保持 | 每命令独立 |
| Elements 数量 | 正常 | 正常 |
| locator 查找 | 通过 Daemon IPC | 直接本地执行 |
| 错误传播 | 可能超时 | 直接返回 |

### 测试建议
1. 先在本地模式验证命令逻辑正确
2. 再在 Daemon 模式测试跨命令状态保持
3. 注意两种模式下的 timeout 设置可能不同

---

## 21. 常见错误速查

| 错误信息 | 可能原因 | 解决思路 |
|---------|---------|---------|
| `session "default" not found` | Session 未创建或已关闭 | 使用 `open` 创建 session |
| `Elements` 为空 | 页面未加载完成 / JS 错误 | 检查 HTTP 服务器 / 用 `eval` 验证 |
| `i/o timeout` | Playwright 操作阻塞 | 检查 locator / 添加调试日志 |
| `failed to connect to daemon` | Daemon 未运行 | `go-browser.exe start` |
| 导航无反应 | URL 无效 / 网络问题 | 检查 URL 可访问性 |

---

## 总结

| 经验点 | 关键教训 |
|--------|---------|
| CLI 架构限制 | 需要守护进程实现状态保持 |
| Windows socket | 使用 TCP 代替 Unix socket |
| PID 获取 | 无法直接获取，使用 IsConnected() |
| Flag 传递 | 通过环境变量或配置文件 |
| 协议设计 | JSON-RPC 简洁高效 |
| 异步关闭 | 先响应后清理，异步调用避免死锁 |
| 启动就绪 | 轮询检测而非固定延迟 |
| IPC 架构 | 所有命令优先通过 IPC，回退本地模式 |
| 跨平台差异 | Windows 超时需要更长时间 |
| 幂等设计 | 停止/清理函数必须可多次调用 |
| 多信号响应 | 同时监听系统信号和内部 channel |
| 自愈机制 | 定期检测 + 自动重连 |
| 测试环境 | 简单 HTML 页面更适合测试 |
| 参数传递 | 命令行引号容易被忽略 |

## 添加 Daemon 命令支持的标准模式

### 问题描述
新增命令（如 `pdf`）或修复已有命令（如 `screenshot`）时，需要在多个文件中添加支持：

1. `protocol.go` - 定义方法名和结果类型
2. `server.go` - 实现服务端处理器
3. `client.go` - 添加客户端调用方法
4. `cmd/commands/*.go` - 在命令中使用 daemon 模式

### 典型错误：PDF 调用 Screenshot
```go
// 错误：pdfCmd 中调用了 client.Screenshot()
result, err := client.Screenshot(cmd.GetSessionName())

// 正确：应该调用 client.Pdf()
result, err := client.Pdf(cmd.GetSessionName())
```

### 最小化修改原则
- 优先使用 `daemonMode()` 而非手动展开检查条件
- 使用 `printDaemonSnapshot()` 而非重复打印逻辑
- 添加新命令时，Daemon 分支调用 client 新方法
- 确保本地模式 fallback 正确工作

## 使用辅助函数避免重复代码

### daemonMode()
```go
func daemonMode() bool {
    return daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == ""
}
```
所有命令应使用此函数，而非手动展开4条件检查。

### printDaemonSnapshot()
```go
func printDaemonSnapshot(formatter *output.Formatter, client *daemon.Client, sessionName string) error {
    snapshotResult, err := client.Snapshot(sessionName)
    if err != nil {
        return err
    }
    if !snapshotResult.Success || snapshotResult.Snapshot == nil {
        return fmt.Errorf("daemon snapshot failed")
    }
    fmt.Print(formatter.FormatSnapshot(daemonSnapshotToSnapshot(snapshotResult.Snapshot)))
    return nil
}
```
Daemon 模式成功后应调用此函数获取快照输出。

### daemonSnapshotToSnapshot()
转换 daemon 快照格式到本地格式，供 Formatter 使用。

## 命令支持状态速查

### 已支持 Daemon
| 命令 | 备注 |
|------|------|
| open/goto/snapshot/reload | 导航相关 |
| go-back/go-forward | 导航相关 |
| click/fill/hover | 元素操作 |
| eval/type/press/keydown/keyup | 键盘操作 |
| mousemove/mousedown/mouseup/mousewheel | 鼠标操作 |
| tab-list/new/close/select | 标签页操作 |
| screenshot/pdf | 媒体操作 |
| list/close | 会话管理 |
| dblclick/check/uncheck/select/drag | 高级元素操作 |
| dialog-accept/dialog-dismiss | 对话框操作 |
| state-save/state-load | 状态持久化 |
| cookie-list/get/set/delete/clear | Cookie 操作 |
| localstorage-list/get/set/delete/clear | localStorage 操作 |

### 不支持 Daemon（会报 session not found）
| 命令 | 备注 |
|------|------|
| sessionstorage-* | 需要实现 |
| run-code | 功能未实现 |

### 功能未实现
| 命令 | 备注 |
|------|------|
| console/network/route | 功能未实现 |
| tracing-*/video-* | 功能未实现 |
| attach | 需要特殊标志 |

## 相关文档

- [Daemon 命令支持问题](daemon-command-support.md) - 命令支持不完整问题的根因分析和修复方案
- [会话直通测试用例](../test/session-through-test.md) - 守护进程管理、会话管理、页面导航等核心测试
- [元素操作直通测试用例](../test/element-operations-test.md) - 双击、选择、复选框、拖拽、上传等元素操作测试
