# Claude Code Handoff: go-browser Follow-up Work

Last updated: 2026-04-21

This document is written as an implementation handoff for Claude Code. It focuses on the next concrete engineering tasks after the current daemon IPC improvements.

## Current Baseline

The CLI is most reliable through the daemon flow:

```bash
go-browser start
go-browser open https://example.com
go-browser snapshot
go-browser stop
```

The following daemon-backed commands have been implemented and smoke-tested:

- `start`, `status`, `stop`
- `open`, `goto`, `snapshot`, `close`, `list`
- `go-back`, `go-forward`, `reload`
- `click`, `fill`, `hover`, `eval`, `resize`
- `type`, `press`, `keydown`, `keyup`
- `mousemove`, `mousedown`, `mouseup`, `mousewheel`
- `tab-list`, `tab-new`, `tab-close`, `tab-select`

Snapshot refs such as `e0` and `e1` work in daemon-backed `click`, `fill`, and `hover`. The implementation stores internal selectors in the daemon's per-session ref cache. Re-run `snapshot` after navigation or major DOM changes before relying on old refs.

Recent fixes already applied:

- Nested DOM snapshots no longer duplicate refs for the same element.
- Daemon navigation, reload, snapshot, and screenshot operate on the selected tab.
- `run-code` validates input and supports `--filename`.
- Placeholder console/network/tracing/video commands return clear not-implemented errors.

Before starting work, run:

```bash
go test ./...
go vet ./...
go build -o go-browser .
./go-browser status
```

Note: this workspace may not have Git metadata available from `/mnt/c/work/go-cli-browser`, so do not depend on `git status` as the only source of truth.

## Implementation Pattern

For each new daemon-backed command:

1. Add params and method constants in `internal/daemon/protocol.go`.
2. Add a client method in `internal/daemon/client.go`.
3. Add a server handler and route it in `internal/daemon/server.go`.
4. Update the Cobra command to prefer daemon mode when `daemonMode()` is true.
5. Keep local fallback behavior for non-daemon usage.
6. Return daemon errors directly instead of silently falling back.
7. Add small unit tests for parsing/helpers where possible.
8. Run static checks and at least one real daemon smoke test.

Existing helper functions worth reusing:

- `daemonMode()` in `cmd/commands/core.go`
- `printDaemonSnapshot(...)` in `cmd/commands/core.go`
- `normalizeTargetURL(...)` in `cmd/commands/core.go`
- `Server.resolveLocator(...)` in `internal/daemon/server.go`
- `Server.pageForSession(...)` and `Server.pageAndHandleForSession(...)` in `internal/daemon/server.go`

## Priority 1: Daemon-back Remaining Element Actions

Commands:

- `dblclick`
- `select`
- `check`
- `uncheck`
- `drag`
- `upload`

Why first:

- They are already implemented locally in `cmd/commands/core.go`.
- They are user-facing interactive commands.
- Most can reuse `Server.resolveLocator(...)`, which already supports CSS selectors and snapshot refs.

Files:

- `cmd/commands/core.go`
- `internal/daemon/protocol.go`
- `internal/daemon/client.go`
- `internal/daemon/server.go`
- `cmd/commands/core_test.go` if helper tests are added

Suggested protocol structs:

```go
type SelectParams struct {
    SessionName string `json:"session_name,omitempty"`
    Locator     string `json:"locator"`
    Value       string `json:"value"`
}

type DragParams struct {
    SessionName   string `json:"session_name,omitempty"`
    SourceLocator string `json:"source_locator"`
    TargetLocator string `json:"target_locator"`
}

type UploadParams struct {
    SessionName string `json:"session_name,omitempty"`
    Locator     string `json:"locator,omitempty"`
    FilePath    string `json:"file_path"`
}
```

Implementation notes:

- `dblclick`, `select`, `check`, and `uncheck` should use `resolveLocator`.
- `drag` should resolve both source and target locators through `resolveLocator`.
- `upload` currently always uses `input[type=file]`; keep that default for backward compatibility, but consider accepting an optional locator later.
- For `select`, pass `playwright.SelectOptionValues{Values: &[]string{value}}`.
- After each successful action, command code should call `printDaemonSnapshot`.

Acceptance smoke:

```bash
./go-browser stop >/tmp/go-browser-stop.log 2>&1 || true
./go-browser start
./go-browser open 'data:text/html,<button id="btn" ondblclick="document.body.dataset.dbl=1">Double</button><select id="sel"><option value="a">A</option><option value="b">B</option></select><input id="cb" type="checkbox">'
./go-browser dblclick '#btn'
./go-browser select '#sel' b
./go-browser check '#cb'
./go-browser eval 'JSON.stringify({dbl: document.body.dataset.dbl, selected: document.querySelector("#sel").value, checked: document.querySelector("#cb").checked})'
./go-browser uncheck '#cb'
./go-browser stop
```

Expected eval value should include:

- `dbl: "1"`
- `selected: "b"`
- `checked: true` before uncheck

Ref smoke:

```bash
./go-browser start
./go-browser open 'data:text/html,<select id="sel"><option value="a">A</option><option value="b">B</option></select><input id="cb" type="checkbox">'
./go-browser snapshot
./go-browser select e0 b
./go-browser check e1
./go-browser eval 'document.querySelector("#sel").value + "|" + document.querySelector("#cb").checked'
./go-browser stop
```

## Priority 2: Daemon-back Media Commands

Commands:

- `screenshot`
- `pdf`

Why next:

- These are high-value and relatively isolated.
- `internal/daemon/server.go` already has a basic `handleScreenshot`, but the command in `cmd/commands/media.go` does not use daemon mode yet.

Files:

- `cmd/commands/media.go`
- `internal/daemon/protocol.go`
- `internal/daemon/client.go`
- `internal/daemon/server.go`

Implementation notes:

- Add params for filename/path and optional element locator.
- Wire `screenshot` command to daemon mode.
- Current server screenshot handler always uses an auto filename. Extend it to honor `--filename`.
- Decide whether element screenshot should be implemented now. If yes, resolve the optional locator and call locator screenshot if Playwright Go supports it. If not, return a clear error for element screenshot in daemon mode rather than silently taking a full-page screenshot.
- Add a daemon `pdf` method. Chromium-only limitations should surface as real Playwright errors.
- After screenshot/pdf, printing a snapshot is useful but not mandatory. Preserve current CLI behavior if practical.

Acceptance smoke:

```bash
./go-browser stop >/tmp/go-browser-stop.log 2>&1 || true
./go-browser start
./go-browser open 'data:text/html,<title>Media</title><h1>Hello</h1>'
./go-browser screenshot --filename /tmp/go-browser-smoke.png
test -s /tmp/go-browser-smoke.png
./go-browser pdf --filename /tmp/go-browser-smoke.pdf
test -s /tmp/go-browser-smoke.pdf
./go-browser stop
```

## Priority 3: Implement Placeholder Commands For Real

The first honesty pass is complete: these commands now return clear not-implemented errors instead of fake success.

Commands still needing real implementations:

- `console`
- `network`
- `tracing-start`
- `tracing-stop`
- `video-start`
- `video-stop`
- `video-chapter`

Why:

- False-positive success is worse than a clear unsupported error.
- This keeps the CLI trustworthy while implementation continues.

Files:

- `cmd/commands/devtools.go`
- Any tracing/video command files if separate
- `docs/go-browser/SKILL.md`
- `docs/go-browser/next-steps.md`

Recommended next pass:

- Implement real daemon-backed behavior when ready.
- Keep command registration so users get a clear message instead of an unknown command until then.

Acceptance:

```bash
./go-browser console
./go-browser network
./go-browser tracing-start
```

Each should fail clearly and not claim success.

## Priority 4: Daemon-back Storage Commands

Commands:

- `state-save`
- `state-load`
- `cookie-list`, `cookie-get`, `cookie-set`, `cookie-delete`, `cookie-clear`
- `localstorage-list`, `localstorage-get`, `localstorage-set`, `localstorage-delete`, `localstorage-clear`
- `sessionstorage-list`, `sessionstorage-get`, `sessionstorage-set`, `sessionstorage-delete`, `sessionstorage-clear`

Files:

- `cmd/commands/storage.go`
- `internal/daemon/protocol.go`
- `internal/daemon/client.go`
- `internal/daemon/server.go`
- `docs/go-browser/references/storage-state.md`

Implementation notes:

- `state-save` can call `BrowserContext.StorageState`.
- `state-load` is trickier because an existing context may need to be recreated with storage state. Be explicit about behavior.
- Cookie operations should use browser context cookies APIs.
- local/session storage operations should evaluate JS on the current page.
- Be careful with named sessions.

Suggested order:

1. `cookie-list` and `cookie-clear`.
2. `localstorage-*` and `sessionstorage-*`.
3. `state-save`.
4. `state-load` after deciding context recreation semantics.

Acceptance smoke:

```bash
./go-browser stop >/tmp/go-browser-stop.log 2>&1 || true
./go-browser start
./go-browser open 'data:text/html,<title>Storage</title>'
./go-browser eval 'localStorage.setItem("k", "v"); sessionStorage.setItem("s", "t"); document.cookie = "a=b"'
./go-browser localstorage-get k
./go-browser sessionstorage-get s
./go-browser cookie-list
./go-browser stop
```

## Priority 5: Daemon-back Network Route Commands

Commands:

- `route`
- `route-list`
- `unroute`

Files:

- `cmd/commands/network.go`
- `internal/daemon/protocol.go`
- `internal/daemon/client.go`
- `internal/daemon/server.go`
- `docs/go-browser/references/request-mocking.md`

Implementation notes:

- Route handlers must live in the daemon process because that process owns the browser context/page.
- Maintain a per-session route registry in `BrowserHandle` so `route-list` can report active mocks.
- Decide whether routes attach to page or context. Context-level routing is usually more durable across tabs.
- Keep unsupported older flags explicit:
  - `route --content-type`
  - `route --remove-header`

Acceptance smoke:

```bash
./go-browser stop >/tmp/go-browser-stop.log 2>&1 || true
./go-browser start
./go-browser open 'data:text/html,<script>fetch("/api").then(r=>r.text()).then(t=>document.body.dataset.api=t)</script>'
./go-browser route '**/api' --body mocked
./go-browser reload
./go-browser eval 'document.body.dataset.api'
./go-browser route-list
./go-browser unroute '**/api'
./go-browser stop
```

## Priority 6: Make `run-code` Honest or Useful

Current behavior:

- `run-code` is not daemon-backed.
- It calls `page.Evaluate(code, nil)`.
- It does not support the documented `async page => { ... }` style.

Files:

- Locate with `rg "run-code|runCode|RunCode"`.
- `docs/go-browser/references/running-code.md`
- `internal/daemon/protocol.go`
- `internal/daemon/client.go`
- `internal/daemon/server.go`

Options:

1. Document `run-code` as local-only and direct users to `eval`.
2. Daemon-back `run-code` as expression evaluation, equivalent to `eval`.
3. Implement a wrapper for async snippets if feasible.

Recommended first pass:

- Either daemon-back it as expression evaluation or return a clear message that `eval` is the supported daemon command.

## Priority 7: Flag and Documentation Alignment

Known unsupported flags from older docs:

- `cookie-list --domain`
- `cookie-list --path`
- `cookie-set --path`
- `cookie-set --sameSite`
- `cookie-set --expires`
- `route --content-type`
- `route --remove-header`
- `video-chapter --description`
- `video-chapter --duration`
- `go-browser --version`

Already implemented:

- Global `--headed` as an alias for `--headless=false`.

Recommended approach:

- Only implement flags that have clear behavior and tests.
- Otherwise keep docs explicit that they are unsupported.
- If adding `go-browser --version`, use a package variable so releases can set it with `-ldflags`.

## Regression Test Checklist

Run this before handing back:

```bash
go test ./...
go vet ./...
go build -o go-browser .
./go-browser status
```

Run this after daemon command changes:

```bash
./go-browser stop >/tmp/go-browser-stop.log 2>&1 || true
./go-browser start
./go-browser open 'data:text/html,<title>Smoke</title><button id="btn">Click</button><input id="name">'
./go-browser snapshot
./go-browser stop
```

Existing verified smokes:

```bash
# Snapshot refs
./go-browser stop >/tmp/go-browser-stop.log 2>&1 || true
./go-browser start
./go-browser open 'data:text/html,<button id="btn" onclick="document.body.dataset.clicked=1">Click</button><input id="name">'
./go-browser click e0
./go-browser fill e1 abc
./go-browser eval 'document.body.dataset.clicked + "|" + document.querySelector("#name").value'
./go-browser stop
```

```bash
# Keyboard and mouse
cat > /tmp/go-browser-input-smoke.html <<'HTML'
<!doctype html>
<html>
<body style="height: 2000px; margin: 0;">
  <input id="name" autofocus>
  <div id="box" style="width: 200px; height: 120px; background: #ddd; margin-top: 20px;">box</div>
  <script>
    document.addEventListener('mousemove', e => document.body.dataset.move = `${e.clientX},${e.clientY}`);
    document.addEventListener('mousedown', e => document.body.dataset.down = e.button);
    document.addEventListener('mouseup', e => document.body.dataset.up = e.button);
    document.addEventListener('wheel', e => document.body.dataset.wheel = `${e.deltaX},${e.deltaY}`);
    document.addEventListener('keydown', e => document.body.dataset.keydown = e.key);
    document.addEventListener('keyup', e => document.body.dataset.keyup = e.key);
  </script>
</body>
</html>
HTML
./go-browser start
./go-browser open /tmp/go-browser-input-smoke.html
./go-browser click '#name'
./go-browser type 'abc'
./go-browser press Backspace
./go-browser keydown Shift
./go-browser keyup Shift
./go-browser mousemove 20 30
./go-browser mousedown left
./go-browser mouseup left
./go-browser mousewheel 0 120
./go-browser eval 'JSON.stringify({value: document.querySelector("#name").value, move: document.body.dataset.move, down: document.body.dataset.down, up: document.body.dataset.up, wheel: document.body.dataset.wheel, keydown: document.body.dataset.keydown, keyup: document.body.dataset.keyup})'
./go-browser stop
```

```bash
# Tabs
./go-browser stop >/tmp/go-browser-stop.log 2>&1 || true
./go-browser start
./go-browser open 'data:text/html,<title>One</title><h1>one</h1>'
./go-browser tab-new 'data:text/html,<title>Two</title><h1>two</h1>'
./go-browser tab-list
./go-browser tab-select 0
./go-browser eval 'document.title'
./go-browser tab-close 1
./go-browser tab-list
./go-browser stop
```

## Documentation To Update After Each Task

- `docs/go-browser/next-steps.md`
- `docs/go-browser/SKILL.md`
- This handoff doc if task priorities change

Keep docs honest: do not document planned behavior as supported until it has a passing command path and smoke test.
