# 守护进程开发经验总结

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

## 7. 调试技巧

### 查看端口占用
```bash
# Windows
netstat -ano | findstr "9223"

# Linux
lsof -i :9223
```

### 查看进程
```bash
# Windows
tasklist | findstr "go-browser"

# 终止进程
taskkill //F //IM go-browser.exe
```

### 检查 socket 文件
```bash
# Linux
ls -la /tmp/go-browser.sock
```

---

## 总结

| 经验点 | 关键教训 |
|--------|----------|
| CLI 架构限制 | 需要守护进程实现状态保持 |
| Windows socket | 使用 TCP 代替 Unix socket |
| PID 获取 | 无法直接获取，使用 IsConnected() |
| Flag 传递 | 通过环境变量或配置文件 |
| 协议设计 | JSON-RPC 简洁高效 |
| 自愈机制 | 定期检测 + 自动重连 |

## 相关文档

- [IPC 调试经验](daemon-ipc-debugging.md) - 异步关闭、启动就绪、IPC 架构、跨平台差异等调试问题
- [Daemon 命令支持问题](daemon-command-support.md) - 命令支持不完整问题的根因分析和修复方案
