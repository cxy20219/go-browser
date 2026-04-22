# Daemon 有头模式 (Headed Mode) 调试经验

## 问题描述

使用 `daemon --headed` 启动守护进程后，通过 `open` 命令打开浏览器时，浏览器窗口没有显示出来，一直是 headless 模式。

## 根因分析

1. **命令行标志传递链路**：
   - `daemon --headed` → 设置 `daemonHeaded` 变量
   - 但这个变量只影响 daemon 进程的启动参数
   - 并没有传递给 `daemon.NewServer()` 创建 server 实例
   - server 实例不知道应以什么模式（headless/hebrowse）启动浏览器

2. **客户端请求时的处理**：
   - `open` 命令调用 `client.Open(sessionName, url, opts.Browser, opts.Headless)`
   - 客户端传递的是命令行的 `--headless` 标志，不是 daemon 的模式
   - `handleOpen` 使用 `params.Headless` 启动浏览器，忽略了 daemon 本身的设置

3. **Server 结构体缺少状态**：
   - `Server` 结构体没有存储 `headless` 字段
   - 无法记住 daemon 启动时是 headless 还是 headed 模式

## 解决方案

### 1. 修改 Server 结构体

```go
type Server struct {
    // ... 现有字段
    headless bool // true = headless, false = headed
}
```

### 2. 修改 NewServer 函数

```go
func NewServer(headless bool) (*Server, error) {
    s := &Server{
        // ... 现有初始化
        headless: headless,
    }
    return s, nil
}
```

### 3. 修改 daemon 命令传递 headless 标志

```go
var daemonCmd = &cobra.Command{
    RunE: func(c *cobra.Command, args []string) error {
        headless := !daemonHeaded  // headed 模式对应的 headless=false
        server, err := daemon.NewServer(headless)
        // ...
    },
}
```

### 4. 修改 handleOpen 使用 daemon 的 headless 设置

```go
func (s *Server) handleOpen(paramsJSON json.RawMessage) (*Result, error) {
    // ...
    // 使用 daemon 的 headless 设置，而非客户端传递的参数
    headless := s.headless

    browserImpl := browser.NewLocalBrowser()
    result, err := browserImpl.Launch(&session.SessionOptions{
        Browser:  browserType,
        Headless: headless,
    })
    // ...
}
```

## 经验总结

### 1. 守护进程模式下的配置传递

守护进程模式与直接命令模式不同：
- **直接模式**：每次命令都启动新的浏览器实例，flags 直接影响本次启动
- **守护模式**：daemon 启动时确定浏览器配置，所有后续操作复用同一个浏览器

配置传递链路：
```
命令行 flags → daemon 进程 → daemon.Server.headless → handleOpen → browser.Launch
```

### 2. 关键设计原则

**Daemon 应该管理浏览器生命周期**：
- daemon 启动时确定的模式（headless/headed）应该是权威设置
- 客户端的 `--headless` 标志在 daemon 模式下应该被忽略，或者用于明确覆盖
- 保持 daemon 所有会话的浏览器模式一致性

### 3. 代码组织建议

将 headless 状态封装在 Server 中：

```go
// good: headless 是 server 的内部状态
server, _ := daemon.NewServer(!headedFlag)
server.Start()

// bad: 每个操作都需要传递 headless 参数
server.Open(url, headlessFlag)
```

### 4. 排查技巧

当发现 daemon 行为与预期不符时：

1. **检查 daemon 进程是否真的在运行**：
   ```bash
   go-browser status
   ```

2. **检查 daemon 启动时的参数**：
   - 如果用 `daemon --headed` 启动，浏览器应该是 headed 模式
   - 如果用 `daemon`（无参数）启动，默认是 headless 模式

3. **客户端 flags 在 daemon 模式下无效**：
   - `open --headless=false` 在 daemon 模式下不会打开有头浏览器
   - 需要重启 daemon 并使用正确的 flags

### 5. 相关文件

- `cmd/daemon.go` - daemon 命令入口
- `internal/daemon/server.go` - Server 结构体和 NewServer
- `internal/daemon/protocol.go` - OpenParams 定义
- `cmd/commands/core.go` - open 命令实现

## 预防措施

1. **在添加 daemon 支持时**，确保 daemon 级别的配置（如 headless/headed）被正确存储和传递
2. **创建 daemon 相关 PR 时**，测试用例应覆盖 headless 和 headed 两种模式
3. **文档化 daemon 与本地模式的差异**，避免用户困惑
