## 这是 Browserless 的连接文档
https://docs.browserless.io/baas/connection-url-patterns

## 参考go-browser的命令以及使用skill
[go-browser 命令参考文档](docs/go-browser/references/)
[go-browser skill 文档](docs/go-browser/SKILL.md)

## 测试目录
`test/` 目录是浏览器测试页面
- [index.html](test/index.html) - 主入口页面
- [app.js](test/app.js) - Vue 应用入口
- [components/](test/components/) - 20 个测试组件

## 项目规划文档
[实现计划](docs/plan.md)

## 直通测试文档
[会话直通测试用例](docs/harness/test/session-through-test.md) - 覆盖守护进程管理、会话管理、页面导航、标签页管理、媒体操作、边界条件、异常情况等 36 个测试用例。基于守护进程架构设计。

[元素操作直通测试用例](docs/harness/test/element-operations-test.md) - 覆盖双击、选择、下拉框操作、复选框操作、拖拽、文件上传等 26 个测试用例。包括 dblclick、select、check、uncheck、drag、upload 命令的 daemon 模式测试。

## 开发经验文档
[守护进程开发经验](docs/harness/experience/daemon-development.md) - 涵盖 CLI 架构限制、Windows socket 差异、Playwright PID 问题、Flag 传递、JSON-RPC 协议设计、异步关闭竞态、启动就绪检测、CLI 命令绕过守护进程、跨平台超时差异、优雅关闭设计、自愈机制、测试环境问题（ES modules 支持）、命令行引号问题、I/O timeout 排查、Daemon 与本地模式差异、调试技巧、常见错误速查。**新增**：添加 Daemon 命令支持的标准模式（protocol/server/client/commands 四层修改）、使用辅助函数避免重复代码（daemonMode/printDaemonSnapshot/daemonSnapshotToSnapshot）、命令支持状态速查表。

[Daemon 命令支持不完整问题](docs/harness/experience/daemon-command-support.md) - 部分命令（click、fill、tab-*、screenshot 等）不支持 daemon 模式导致 "session not found" 错误的根因分析、修复方案（标准 6 步模式）、文件编辑风险提示及验证方法。

[Daemon 开发调试经验](docs/harness/experience/daemon-debugging-lessons.md) - 类型转换坑点（int64/float64、Expires 字段）、API 设计一致性原则、外部类型直接使用、复制粘贴错误示例、命令分支结构模式、Playwright API 限制、JSON 序列化格式选择、文件编辑风险提示。

## 要求
- 开发前看一下相关文档
- 开发完成后严格测试
- 根据开发和测试反馈更新文档

## 目标平台
- Windows
- macOS
- Linux