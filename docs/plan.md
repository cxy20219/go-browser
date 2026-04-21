# Go CLI Browser - 实现计划

## Context

需要使用 Go 语言实现一个浏览器自动化 CLI 工具，支持本地浏览器和 Browserless 云浏览器两种运行模式。命令参考 docs/commands.md，主要给 Agent 使用，需要会话管理功能，使用 playwright-go + Cobra 方案。

## 技术方案

- **语言**: Go 1.21+
- **浏览器自动化**: github.com/playwright-community/playwright-go
- **CLI框架**: github.com/spf13/cobra
- **连接模式**:
  - 本地: `pw.Chromium.Launch()` 等 → 返回 `*BrowserContext`
  - 远程: `pw.Chromium.Connect(url, options)` → 返回 `*BrowserContext`
  - 附加模式: `pw.Chromium.ConnectViaCDP(url)` → 返回 `*BrowserContext`

**重要**: `BrowserType.Connect` 返回的是 `*BrowserContext`，不是 `playwright.Browser`。需要通过 `BrowserContext.NewPage()` 或 `BrowserContext.Pages()` 获取 `Page`。

```
go-cli-browser/
├── cmd/
│   ├── root.go              # 根命令，全局flags (-s, --browser, --headless, --remote)
│   └── commands/
│       ├── core.go          # open, goto, close, type, click, fill, snapshot, eval, dialog-*, resize
│       ├── navigation.go    # go-back, go-forward, reload
│       ├── keyboard.go      # press, keydown, keyup
│       ├── mouse.go         # mousemove, mousedown, mouseup, mousewheel
│       ├── media.go         # screenshot, pdf
│       ├── tabs.go          # tab-list, tab-new, tab-close, tab-select
│       ├── storage.go       # state-save/load, cookie-*, localstorage-*, sessionstorage-*
│       ├── network.go       # route, route-list, unroute
│       ├── devtools.go      # console, network, run-code, tracing-*, video-*
│       └── session.go       # list, close-all, kill-all
├── internal/
│   ├── session/
│   │   ├── manager.go       # SessionManager - 线程安全会话存储
│   │   └── session.go      # Session 结构体, SessionOptions
│   ├── browser/
│   │   ├── browser.go      # Browser 接口定义
│   │   ├── local.go        # LocalBrowser 实现 (Launch)
│   │   ├── remote.go       # RemoteBrowser 实现 (Connect to Browserless)
│   │   └── attach.go       # AttachBrowser 实现 (ConnectViaCDP, extension)
│   └── snapshot/
│   │   ├── snapshot.go      # 快照生成，元素ref映射
│   │   └── ref.go           # RefGenerator 生成短引用 (e0, e1, ea...)
│   ├── locator/
│   │   └── locator.go       # 解析 ref/selector 到 Playwright Locator
│   └── output/
│       └── output.go        # 一致性输出格式化
├── main.go
└── go.mod
```

## 关键实现

### 1. 会话管理 (internal/session/)

```go
type Session struct {
    Name       string              // 会话名称
    Context    *BrowserContext     // Playwright BrowserContext (Connect/Launch 返回)
    Pages      []playwright.Page   // 多标签页支持
    CurrentPage int                // 当前活动页面索引
    Mode       BrowserMode         // ModeLocal / ModeRemote / ModeAttached
    ProfileDir string              // 配置文件路径 (用于 delete-data)
    CreatedAt  time.Time
}

type SessionManager struct {
    mu       sync.RWMutex
    sessions map[string]*Session
}
```

- 全局单例 `SessionManager`
- `-s=name` flag 选择会话，默认 "default"
- 线程安全的 map 存储
- 每个会话支持多个标签页 (`Pages`)

### 2. 浏览器连接 (internal/browser/)

```go
type Browser interface {
    Launch(opts *SessionOptions) (*BrowserContext, error)
    Connect(url string, opts *SessionOptions) (*BrowserContext, error)
    ConnectViaCDP(url string) (*BrowserContext, error)  // 用于 attach 命令
}

// LocalBrowser: 使用 Launch() 创建本地浏览器
// RemoteBrowser: 使用 Connect() 连接 Browserless 云
// AttachBrowser: 使用 ConnectViaCDP() 连接已运行的浏览器

// Remote URL 格式: wss://production-sfo.browserless.io/chromium/playwright?token=TOKEN
// CDP URL 格式: ws://localhost:9222 (Chrome DevTools Protocol)
// Channel 格式: --cdp=chrome, --cdp=msedge 等 (通过 CDP endpoint 自动发现)
```

### 3. 元素 Ref 系统 (internal/snapshot/)

- 快照生成时为每个元素分配短引用: `e0, e1, e2 ... ea, eb ...`
- ref 映射到 CSS selector 存储在 `cssPathCache` map
- 命令执行时通过 ref 查找对应元素

### 4. Locator 解析 (internal/locator/)

```go
func (r *LocatorResolver) Resolve(ref string) (playwright.Locator, error) {
    // 支持:
    // - CSS selector: "#main > button"
    // - Role locator: "getByRole('button', {name: 'Submit'})"
    // - TestId: "getByTestId('submit-btn')"
    // - Ref 引用: "e15" -> 从缓存查找 CSS path
}
```

## 命令实现清单

| 分组 | 命令 | 关键文件 |
|------|------|----------|
| Core | open, goto, close, type, click, dblclick, fill, fill --submit, drag, hover, select, upload, check, uncheck, snapshot, eval, dialog-accept, dialog-dismiss, resize | cmd/commands/core.go |
| Attach | attach --cdp=chrome, attach --cdp=msedge, attach --cdp=http://..., attach --extension | cmd/commands/attach.go |
| Navigation | go-back, go-forward, reload | cmd/commands/navigation.go |
| Keyboard | press, keydown, keyup | cmd/commands/keyboard.go |
| Mouse | mousemove, mousedown, mousedown right, mouseup, mouseup right, mousewheel | cmd/commands/mouse.go |
| Media | screenshot, screenshot e5, screenshot --filename=, pdf, pdf --filename= | cmd/commands/media.go |
| Tabs | tab-list, tab-new, tab-new url, tab-close, tab-close N, tab-select N | cmd/commands/tabs.go |
| Storage | state-save, state-load, cookie-list, cookie-get, cookie-set, cookie-delete, cookie-clear, localstorage-list/get/set/delete/clear, sessionstorage-list/get/set/delete/clear, delete-data | cmd/commands/storage.go |
| Network | route, route-list, unroute | cmd/commands/network.go |
| DevTools | console, console warning, network, run-code, run-code --filename, tracing-start, tracing-stop, video-start, video-chapter, video-stop | cmd/commands/devtools.go |
| Session | list, close, close-all, kill-all, delete-data | cmd/commands/session.go |

**注意**:
- `open` 命令支持参数: `--browser=chrome/firefox/webkit/msedge`, `--headed`, `--persistent`, `--profile=`, `--config=`
- `attach` 命令用于连接已运行的浏览器，支持 channel/cdp endpoint/extension 三种方式
- `snapshot` 命令支持: `--filename=`, `--depth=N`, `snapshot selector`, `snapshot element-ref`
- `run-code` 支持: `--filename=script.js` 从文件执行
- `console` 命令支持子命令: `console`, `console warning`

## 关键文件清单

1. **internal/session/manager.go** - 会话管理核心
2. **internal/browser/browser.go** - 浏览器接口定义 (Launch/Connect/ConnectViaCDP)
3. **internal/browser/local.go** - 本地浏览器实现
4. **internal/browser/remote.go** - 远程 Browserless 连接
5. **internal/browser/attach.go** - CDP/Extension 附加连接
6. **internal/snapshot/snapshot.go** - 快照和 ref 系统
7. **cmd/root.go** - 根命令和全局 flags (-s, --browser, --headless, --remote, --cdp, --extension)
8. **cmd/commands/core.go** - 核心命令实现
9. **cmd/commands/attach.go** - attach 命令

## 验证方案

1. **构建验证**: `go build -o go-browser .` 成功编译
2. **帮助信息**: `go-browser --help` 显示所有命令
3. **本地浏览器测试**:
   - `go-browser open https://example.com`
   - `go-browser snapshot`
   - `go-browser screenshot`
4. **远程浏览器测试**:
   - `go-browser --remote=wss://... open https://example.com`
5. **附加已运行浏览器测试**:
   - `go-browser attach --cdp=chrome`
   - `go-browser attach --cdp=http://localhost:9222`
   - `go-browser attach --extension`
6. **会话管理测试**:
   - `go-browser -s=session1 open`
   - `go-browser -s=session2 open`
   - `go-browser list`
   - `go-browser -s=session1 close`
7. **多标签页测试**:
   - `go-browser tab-new https://example.com`
   - `go-browser tab-list`
   - `go-browser tab-select 0`
8. **测试用例**: 在 test/ 目录下添加集成测试

## 实现顺序

1. **Phase 1 - 基础框架**: go.mod, cmd/root.go, internal/session/ (Manager + Session), internal/browser/ (接口 + LocalBrowser + RemoteBrowser)
2. **Phase 2 - 核心命令**: open, goto, close, snapshot, click, type, fill (包含 Page 管理和多标签页支持)
3. **Phase 3 - 交互命令**: keyboard, mouse, drag, hover, select
4. **Phase 4 - 导航和媒体**: go-back, go-forward, reload, resize, screenshot, pdf
5. **Phase 5 - 标签页和 Attach**: tab-list, tab-new, tab-close, tab-select, attach 命令
6. **Phase 6 - 存储**: state-save, state-load, cookie-*, localstorage-*, sessionstorage-*, delete-data
7. **Phase 7 - 网络和DevTools**: route, console, network, tracing, video
8. **Phase 8 - 增强功能**: run-code, console 命令增强, 环境变量支持

**关键依赖**:
- Phase 1 必须完成 SessionManager 和 Browser 接口，因为后续所有阶段都依赖会话管理
- Phase 2 开始需要支持多标签页，因为 tab-new 等命令依赖 Context 管理
- Phase 5 的 attach 命令依赖 ConnectViaCDP 方法
