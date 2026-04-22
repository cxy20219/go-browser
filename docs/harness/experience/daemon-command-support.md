# Daemon 架构下命令支持不完整问题

## 问题描述

go-cli-browser 采用守护进程架构，但并非所有命令都支持通过 daemon 发送请求。当 daemon 运行时，不支持 daemon 模式的命令会回退到本地模式，导致无法访问 daemon 中创建的会话。

### 症状

执行命令时报错：`session "default" not found`

```bash
$ go-browser.exe start
Daemon started (PID: xxx)

$ go-browser.exe open https://example.com
### Page
- Page URL: https://example.com/
...

$ go-browser.exe click "#btn"
Error: session "default" not found
```

### 根因分析

每个 CLI 命令（除 daemon 相关命令外）都是独立进程。daemon 模式下：
- `open`、`goto`、`snapshot` 命令已支持 daemon 模式，会通过 IPC 与 daemon 通信
- `click`、`fill`、`hover`、`tab-*`、`screenshot`、`pdf`、`eval` 等命令**未支持 daemon 模式**
- 未支持 daemon 的命令尝试从本地 session manager 获取会话，但 daemon 模式下会话存储在 daemon 进程中

### 已支持 daemon 的命令

| 命令 | 方法名 | 状态 |
|------|--------|------|
| open | MethodOpen | ✅ 支持 |
| goto | MethodGoto | ✅ 支持 |
| snapshot | MethodSnapshot | ✅ 支持 |
| go-back | MethodGoBack | ✅ 支持 |
| go-forward | MethodGoForward | ✅ 支持 |
| reload | MethodReload | ✅ 支持 |
| click | MethodClick | ✅ 支持 |
| fill | MethodFill | ✅ 支持 |
| hover | MethodHover | ✅ 支持 |
| eval | MethodEval | ✅ 支持 |
| type | MethodType | ✅ 支持 |
| press | MethodPress | ✅ 支持 |
| keydown | MethodKeyDown | ✅ 支持 |
| keyup | MethodKeyUp | ✅ 支持 |
| mousemove | MethodMouseMove | ✅ 支持 |
| mousedown | MethodMouseDown | ✅ 支持 |
| mouseup | MethodMouseUp | ✅ 支持 |
| mousewheel | MethodMouseWheel | ✅ 支持 |
| tab-new | MethodTabNew | ✅ 支持 |
| tab-close | MethodTabClose | ✅ 支持 |
| tab-list | MethodTabList | ✅ 支持 |
| tab-select | MethodTabSelect | ✅ 支持 |
| screenshot | MethodScreenshot | ✅ 支持 |
| pdf | MethodPdf | ✅ 支持 |
| dblclick | MethodDblClick | ✅ 支持 |
| check | MethodCheck | ✅ 支持 |
| uncheck | MethodUncheck | ✅ 支持 |
| select | MethodSelect | ✅ 支持 |
| drag | MethodDrag | ✅ 支持 |
| upload | MethodUpload | ✅ 支持 |
| dialog-accept | MethodDialogAccept | ✅ 支持 |
| dialog-dismiss | MethodDialogDismiss | ✅ 支持 |
| state-save | MethodStateSave | ✅ 支持 |
| state-load | MethodStateLoad | ✅ 支持 |
| cookie-list | MethodCookieList | ✅ 支持 |
| cookie-get | MethodCookieGet | ✅ 支持 |
| cookie-set | MethodCookieSet | ✅ 支持 |
| cookie-delete | MethodCookieDelete | ✅ 支持 |
| cookie-clear | MethodCookieClear | ✅ 支持 |
| localstorage-* | MethodLocalStorage | ✅ 支持 |
| sessionstorage-* | MethodSessionStorage | ✅ 支持 |

### 尚未支持 daemon 的命令

| 命令 | 方法名 | 优先级 |
|------|--------|--------|
| run-code | - | 低 |

## 修复方案

为命令添加 daemon 支持的标准模式：

### 1. 在 protocol.go 中添加方法常量

```go
const (
    // ... 已有方法
    MethodClick = "click"
    MethodFill = "fill"
    // ...
)
```

### 2. 在 protocol.go 中添加参数结构体

```go
// ClickParams for click command
type ClickParams struct {
    SessionName string `json:"session_name,omitempty"`
    Selector    string `json:"selector"`
}
```

### 3. 在 server.go 中实现 handler

```go
func (s *Server) handleClick(paramsJSON json.RawMessage) (*Result, error) {
    var params ClickParams
    if err := json.Unmarshal(paramsJSON, &params); err != nil {
        return nil, err
    }

    name := params.SessionName
    if name == "" {
        name = "default"
    }

    s.mu.RLock()
    handle, exists := s.browsers[name]
    s.mu.RUnlock()

    if !exists || handle.Context == nil {
        return nil, fmt.Errorf("session %s not found", name)
    }

    pages := handle.Context.Pages()
    if len(pages) == 0 {
        return nil, fmt.Errorf("no pages in session %s", name)
    }

    page := pages[0]
    // 执行 click 操作
    // ...

    return &Result{
        Success: true,
        Message: "Clicked element",
    }, nil
}
```

### 4. 在 server.go 的 handleRequest 中添加路由

```go
switch req.Method {
case MethodClick:
    result, err = s.handleClick(req.Params)
// ...
}
```

### 5. 在 client.go 中添加客户端方法

```go
func (c *Client) Click(name, selector string) (*Result, error) {
    params := ClickParams{
        SessionName: name,
        Selector:    selector,
    }

    resp, err := c.Send(MethodClick, params)
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
```

### 6. 在命令实现中调用 daemon

```go
var clickCmd = &cobra.Command{
    Use:   "click <locator>",
    Short: "Click an element",
    Args:  cobra.ExactArgs(1),
    RunE: func(c *cobra.Command, args []string) error {
        formatter := output.NewFormatter(cmd.GetRaw())

        // 尝试 daemon 模式
        if daemon.IsDaemonRunning() && cmd.GetCDPURL() == "" && !cmd.GetAttachExt() && cmd.GetRemoteURL() == "" {
            client, err := daemon.NewClient()
            if err == nil {
                defer client.Close()

                result, err := client.Click(cmd.GetSessionName(), args[0])
                if err == nil && result.Success {
                    // 获取 snapshot 并返回
                    snapshotResult, err := client.Snapshot(cmd.GetSessionName())
                    if err == nil && snapshotResult.Success && snapshotResult.Snapshot != nil {
                        snap := &snapshot.Snapshot{
                            URL:   snapshotResult.Snapshot.URL,
                            Title: snapshotResult.Snapshot.Title,
                        }
                        fmt.Print(formatter.FormatSnapshot(snap))
                    }
                    return nil
                }
                // daemon 失败则回退到本地模式
            }
        }

        // 本地模式实现（原始代码）
        // ...
    },
}
```

## 注意事项

### 文件编辑风险

在编辑 `server.go` 和 `client.go` 时，由于文件结构复杂，容易出现以下问题：
- 替换字符串时 old_string 不唯一导致误替换
- 插入位置不正确导致函数嵌套或丢失
- 语法错误（如缺少 `return` 语句）

**建议**：修改前先备份文件，或使用 Write 工具重写整个文件

### daemon 检测条件

命令在决定是否使用 daemon 模式时，应检查：
```go
if daemon.IsDaemonRunning() &&
   cmd.GetCDPURL() == "" &&
   !cmd.GetAttachExt() &&
   cmd.GetRemoteURL() == "" {
    // 使用 daemon
}
```

这确保了特殊连接模式（CDP 附加、远程连接、扩展附加）不会被 daemon 模式干扰。

### 线程安全

访问 `s.browsers` 时必须使用锁：
- 读取用 `s.mu.RLock()` / `s.mu.RUnlock()`
- 写入用 `s.mu.Lock()` / `s.mu.Unlock()`

## 验证方法

修复后重启 daemon 并测试：

```bash
go-browser.exe stop
go build -o go-browser.exe .
go-browser.exe start
go-browser.exe open https://example.com
go-browser.exe click "#btn"  # 不应报错
```

## 相关文件

- `internal/daemon/protocol.go` - 协议定义
- `internal/daemon/server.go` - Daemon 服务端
- `internal/daemon/client.go` - CLI 客户端
- `cmd/commands/core.go` - 核心命令实现
- `cmd/commands/navigation.go` - 导航命令（已支持 daemon）
