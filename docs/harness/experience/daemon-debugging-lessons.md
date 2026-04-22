# Daemon 模式开发调试经验

本文档记录在实现 daemon 命令支持过程中遇到的典型问题和排查经验。

## 1. 类型转换坑点

### int64 vs float64 的 Expires 字段

**问题现象**：编译报错 `cannot use c.Expires (variable of type float64) as int64 value`

**根因**：`playwright.Cookie.Expires` 是 `float64` 类型，而自定义的 `CookieInfo.Expires` 使用了 `int64`。JSON 反序列化时，JavaScript 的数字被解析为 `float64`。

**正确做法**：
```go
// 错误：直接赋值
cookieInfos[i] = CookieInfo{
    Expires: c.Expires,  // float64 -> int64 编译错误
}

// 正确：显式转换
cookieInfos[i] = CookieInfo{
    Expires: int64(c.Expires),
}
```

**排查方法**：编译时直接报错，检查类型定义和实际使用时的一致性。

---

## 2. API 设计一致性

### 参数顺序不一致

**问题现象**：`CookieGet(name, sessionName)` 和 `CookieSet(sessionName, name, ...)` 参数顺序不同

**根因**：实现时没有统一约定，导致调用方容易出错。

**正确做法**：同类 API 参数顺序应保持一致。
```go
// 推荐：sessionName 统一作为第一个参数
func (c *Client) CookieGet(sessionName, name string) (*Result, error)
func (c *Client) CookieDelete(sessionName, name string) (*Result, error)
func (c *Client) CookieSet(sessionName, name, value string, ...) (*Result, error)
```

**教训**：定义 API 时先确定参数顺序规范，避免后续不一致。

---

## 3. 外部类型直接使用

### handleStateLoad 中的 state 解析

**问题现象**：使用 `map[string]interface{}` 手动提取 JSON 字段，代码冗长且易错

**错误做法**：
```go
var state map[string]interface{}
json.Unmarshal(data, &state)
cookiesJSON, ok := state["cookies"].([]interface{})
for _, c := range cookiesJSON {
    if cMap, ok := c.(map[string]interface{}); ok {
        cookie := playwright.OptionalCookie{}
        if name, ok := cMap["name"].(string); ok {  // 重复 6+ 次
            cookie.Name = name
        }
        // ...
    }
}
```

**正确做法**：直接使用 `playwright.StorageState` 类型。
```go
var state playwright.StorageState
if err := json.Unmarshal(data, &state); err != nil {
    return nil, fmt.Errorf("failed to parse state file: %w", err)
}
cookies := make([]playwright.OptionalCookie, len(state.Cookies))
for i, c := range state.Cookies {
    cookies[i] = playwright.OptionalCookie{
        Name:  c.Name,
        Value: c.Value,
        // ...
    }
}
```

**原则**：优先使用外部库提供的类型，而非手动解析。

---

## 4. 容易出错的模式

### 复制粘贴错误

**典型错误**：`pdfCmd` 调用了 `client.Screenshot()` 而非 `client.Pdf()`

```go
// 错误：复制了 screenshot 的代码但没改方法名
result, err := client.Screenshot(cmd.GetSessionName())

// 正确：
result, err := client.Pdf(cmd.GetSessionName())
```

**教训**：复制粘贴后务必检查所有引用是否已更新。

---

## 5. 代码组织模式

### 命令的 daemon 模式分支结构

每个支持 daemon 的命令应遵循统一结构：

```go
var someCmd = &cobra.Command{
    Use:   "some <arg>",
    Args:  cobra.ExactArgs(1),
    RunE: func(c *cobra.Command, args []string) error {
        formatter := output.NewFormatter(cmd.GetRaw())

        // 1. Daemon 模式优先
        if daemonMode() {
            client, err := daemon.NewClient()
            if err != nil {
                return err
            }
            defer client.Close()

            result, err := client.SomeOp(cmd.GetSessionName(), args[0])
            if err != nil {
                return err
            }
            if !result.Success {
                return fmt.Errorf("daemon some failed: %s", result.Message)
            }
            return printDaemonSnapshot(formatter, client, cmd.GetSessionName())
        }

        // 2. 本地模式 fallback
        sess, err := cmd.GetSession()
        if err != nil {
            return err
        }
        // 本地实现...
    },
}
```

**要点**：
- `daemonMode()` 检查放前面，优先使用 daemon
- `defer client.Close()` 确保资源释放
- 失败时回退本地模式，而非直接返回错误
- 成功后调用 `printDaemonSnapshot()` 输出状态

---

## 6. Playwright API 注意事项

### BrowserContext.Cookies() 无单条查询

**问题**：`handleCookieGet` 需要查询单个 cookie，但 `Cookies()` 返回所有 cookies。

**当前实现**：获取所有 cookies 后在 Go 层过滤。
```go
cookies, err := handle.Context.Cookies()
for _, c := range cookies {
    if c.Name == params.Name {
        return &Result{...}, nil
    }
}
```

**说明**：这是 Playwright API 限制，非 bug。

---

## 7. JSON 序列化格式

### State 保存的缩进问题

**观察**：`handleStateSave` 使用 `json.MarshalIndent` 而非 `json.Marshal`。

```go
data, err := json.MarshalIndent(state, "", "  ")
```

**考虑**：对于持久化到文件的场景，缩进有助于人类阅读和调试，但会消耗额外 CPU。对于频繁操作可以改用紧凑格式。

---

## 8. 文件编辑风险提示

### 批量修改时的字符串唯一性

**风险**：在 `server.go` 等大文件中，使用 `replace_all` 可能导致意外替换。

**安全做法**：
- 包含足够的上下文（前后 3+ 行）
- 修改前先 `grep` 确认字符串唯一性
- 大改动前备份文件

---

## 相关文档

- [守护进程开发经验](daemon-development.md) - 架构设计、协议设计、常见错误速查
- [Daemon 命令支持不完整问题](daemon-command-support.md) - 命令支持状态和修复方案
