# go-browser

Go 编写的浏览器自动化 CLI 工具，支持本地浏览器和 Browserless 云浏览器两种运行模式。

## 功能特性

- **多浏览器支持**: Chromium, Firefox, WebKit, Microsoft Edge
- **多种连接模式**: 本地启动、远程连接、附加到已运行浏览器
- **会话管理**: 支持多会话并行，每个会话支持多标签页
- **元素引用系统**: 快照生成短引用 (e0, e1, ea...) 方便交互
- **完整命令集**: 页面导航、元素操作、键盘鼠标模拟、存储管理、网络拦截等

## 安装

```bash
go build -o go-browser .
```

## 快速开始

```bash
# 打开网页
go-browser open https://example.com

# 页面交互
go-browser click "#submit-btn"
go-browser type "#search" "hello"
go-browser snapshot

# 截图
go-browser screenshot
go-browser screenshot --filename=page.png

# 多标签页
go-browser tab-new https://example.com
go-browser tab-list
go-browser tab-select 0

# 会话管理
go-browser -s=session1 open https://example.com
go-browser -s=session2 open https://example.com
go-browser list
```

## 连接模式

### 本地浏览器 (默认)

```bash
go-browser open https://example.com
go-browser --browser=firefox open https://example.com
```

### 远程 Browserless 云

```bash
go-browser --remote=wss://production-sfo.browserless.io/chromium/playwright?token=TOKEN open https://example.com
```

### 附加到已运行浏览器

```bash
# 通过 Chrome channel
go-browser attach --cdp=chrome

# 通过 CDP endpoint
go-browser attach --cdp=http://localhost:9222

# 通过浏览器扩展
go-browser attach --extension
```

## 命令列表

| 分组 | 命令 | 说明 |
|------|------|------|
| 核心 | open, goto, close, click, dblclick, type, fill, drag, hover, select, upload, check, uncheck, snapshot, eval, dialog-accept, dialog-dismiss, resize | 页面和元素操作 |
| 导航 | go-back, go-forward, reload | 浏览器导航 |
| 键盘 | press, keydown, keyup | 键盘事件 |
| 鼠标 | mousemove, mousedown, mouseup, mousewheel | 鼠标事件 |
| 媒体 | screenshot, pdf | 截图和 PDF |
| 标签页 | tab-list, tab-new, tab-close, tab-select | 多标签页管理 |
| 存储 | state-save, state-load, cookie-*, localstorage-*, sessionstorage-*, delete-data | 状态和存储管理 |
| 网络 | route, route-list, unroute | 网络拦截 |
| DevTools | console, network, run-code, tracing-*, video-* | 开发者工具 |
| 会话 | list, close, close-all, kill-all | 会话管理 |
| 附加 | attach | 附加到已运行浏览器 |

详细命令文档请参考 [docs/go-browser/references/](docs/go-browser/references/)。

## 全局参数

- `-s, --session`: 会话名称，默认 "default"
- `--browser`: 浏览器类型 (chromium/firefox/webkit/msedge)
- `--headless`: 无头模式
- `--remote`: 远程 Browserless URL
- `--cdp`: CDP endpoint 或 channel
- `--extension`: 通过浏览器扩展附加

## 开发

### 项目结构

```
go-browser/
├── cmd/
│   ├── root.go              # 根命令
│   └── commands/            # 命令实现
├── internal/
│   ├── browser/             # 浏览器连接
│   ├── session/             # 会话管理
│   ├── snapshot/            # 快照和元素引用
│   └── locator/             # 元素定位
├── test/                    # 测试页面
└── docs/                    # 文档
```

### 技术栈

- Go 1.22+
- [playwright-go](https://github.com/playwright-community/playwright-go) - 浏览器自动化
- [Cobra](https://github.com/spf13/cobra) - CLI 框架

### 运行测试

```bash
# 本地测试
go-browser open https://example.com

# 测试多标签页
go-browser tab-new https://example.com
go-browser tab-list
```

## 文档

- [实现计划](docs/plan.md)
- [命令参考](docs/go-browser/references/)
- [开发经验](docs/harness/experience/)
- [测试用例](docs/harness/test/)
