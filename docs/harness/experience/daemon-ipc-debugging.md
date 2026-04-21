# 守护进程 IPC 调试经验总结

## 1. 守护进程无法正常停止

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

## 2. 守护进程启动后立即退出

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

## 3. CLI 命令绕过守护进程架构

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

## 4. Windows TCP 连接检测超时

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

## 5. 异步关闭时的竞态条件

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

## 6. daemonCmd 的信号响应

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

## 7. 守护进程优雅关闭设计

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

## 8. 进程间通信检测

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

## 9. 自愈机制设计

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

## 10. 常见调试命令

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

---

## 总结

| 问题类型 | 关键教训 |
|---------|---------|
| 异步关闭 | 先响应后清理，异步调用避免死锁 |
| 启动就绪 | 轮询检测而非固定延迟 |
| IPC 架构 | 所有命令优先通过 IPC，回退本地模式 |
| 跨平台差异 | Windows 超时需要更长时间 |
| 幂等设计 | 停止/清理函数必须可多次调用 |
| 多信号响应 | 同时监听系统信号和内部 channel |
| 自愈机制 | 定期检测 + 自动重连 |

## 相关文档

- [守护进程开发经验](daemon-development-lessons.md) - CLI 架构限制、Playwright PID 问题、Flag 传递等架构设计内容
- [Daemon 命令支持问题](daemon-command-support.md) - 命令支持不完整问题的根因分析和修复方案
