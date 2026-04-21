# go-browser Next Steps

Last updated: 2026-04-21

This document captures the current implementation state and the recommended follow-up work for continuing the project.

For an execution-focused handoff to Claude Code, see:

- [Claude Code handoff](claude-code-handoff.md)

## Current Verified State

The CLI builds and basic static checks pass:

```bash
go test ./...
go vet ./...
go build -o go-browser .
```

The daemon workflow is the reliable path:

```bash
go-browser start
go-browser open https://example.com
go-browser snapshot
go-browser stop
```

Daemon-backed commands currently verified:

- `start`, `status`, `stop`
- `open`, `goto`, `snapshot`, `close`, `list`
- `go-back`, `go-forward`, `reload`
- `click`, `fill`, `hover`, `eval`, `resize`
- `type`, `press`, `keydown`, `keyup`
- `mousemove`, `mousedown`, `mouseup`, `mousewheel`
- `tab-list`, `tab-new`, `tab-close`, `tab-select`

Daemon-backed `click`, `fill`, and `hover` now accept CSS selectors and snapshot refs from the most recent daemon snapshot/open/goto output. Re-run `snapshot` after navigation or major DOM re-rendering before using older refs.

Recent fixes:

- Nested DOM snapshots no longer generate duplicate refs for the same element.
- Daemon navigation, reload, snapshot, and screenshot now use the selected tab instead of always using tab 0.
- `run-code` no longer panics when called without code.
- Placeholder commands now return clear not-implemented errors instead of reporting fake success.

## Current Environment Notes

On the tested WSL environment, Playwright Chromium required user-local Linux libraries because sudo was unavailable. The project now auto-adds this directory if present:

```text
~/.local/playwright-deps/root/usr/lib/x86_64-linux-gnu
```

If running on a clean machine, install Playwright browser assets:

```bash
go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5700.1 install chromium
```

If browser startup complains about missing shared libraries, prefer installing system dependencies normally. The user-local dependency workaround should be treated as a WSL-specific convenience, not the ideal production path.

## Highest Priority Fixes

### 1. Snapshot Ref Resolution in Daemon Mode

Status: completed on 2026-04-21 for daemon-backed `click`, `fill`, and `hover`.

Implemented:

- Snapshots attach an internal selector to each element ref.
- The daemon maintains a per-session ref cache from the latest snapshot.
- Daemon element actions resolve refs before calling Playwright locators.
- CSS selector support remains unchanged.

Original problem:

- Snapshots show refs like `e0`, `e1`, `e15`.
- Current daemon element commands pass locator strings directly to `page.Locator(...)`.
- As a result, commands like `go-browser click e0` and `go-browser fill e1 value` time out.

Remaining follow-up:

1. Reuse the same ref resolver when daemon-backing `select`, `check`, `uncheck`, `dblclick`, `drag`, and `upload`.
2. Consider a non-mutating ref strategy later if `data-go-browser-ref` DOM attributes cause issues on mutation-sensitive pages.

Files to inspect:

- `internal/snapshot/snapshot.go`
- `internal/locator/locator.go`
- `internal/daemon/server.go`
- `internal/daemon/protocol.go`

Acceptance test:

```bash
go-browser start
go-browser open 'data:text/html,<button id="btn">Click</button><input id="name">'
go-browser snapshot
go-browser click e0
go-browser fill e1 abc
go-browser eval 'document.querySelector("#name").value'
go-browser stop
```

### 2. Add Daemon IPC for Remaining Interactive Commands

Problem:

Many commands are registered but fail in normal CLI usage because they read the in-process session manager, while the browser lives in the daemon process.

Completed in this pass:

- Keyboard: `type`, `press`, `keydown`, `keyup`
- Mouse: `mousemove`, `mousedown`, `mouseup`, `mousewheel`
- Tabs: `tab-list`, `tab-new`, `tab-close`, `tab-select`

Commands to daemon-back next:

- Media: `screenshot`, `pdf`
- Element actions: `dblclick`, `select`, `drag`, `upload`, `check`, `uncheck`
- Storage: `state-save`, `state-load`, `cookie-*`, `localstorage-*`, `sessionstorage-*`
- Network: `route`, `route-list`, `unroute`

Files to inspect:

- `cmd/commands/core.go`
- `cmd/commands/keyboard.go`
- `cmd/commands/mouse.go`
- `cmd/commands/tabs.go`
- `cmd/commands/media.go`
- `cmd/commands/storage.go`
- `cmd/commands/network.go`
- `internal/daemon/client.go`
- `internal/daemon/protocol.go`
- `internal/daemon/server.go`

Suggested pattern:

1. Add protocol params and method constants.
2. Add daemon server handler.
3. Add daemon client method.
4. In command code, if daemon is running and no remote/CDP/extension mode is active, call daemon and return real daemon errors instead of falling back silently.
5. Keep local fallback for non-daemon use.

### 3. Fix or Remove Placeholder Features

Status: completed for the first honesty pass. These commands now return clear not-implemented errors instead of placeholder success:

- `console`
- `network`
- `tracing-start`, `tracing-stop`
- `video-start`, `video-stop`, `video-chapter`

Future options:

1. Implement real daemon-backed behavior.
2. Mark as experimental in help text.

## Medium Priority Fixes

### 4. Align Flags with Documentation or Keep Docs Strict

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

Implemented:

- `--headed` is now a global convenience alias for `--headless=false`.

Decide per flag whether to implement or keep documentation explicit that it is unsupported.

### 5. Improve URL and File Handling Across Commands

Already improved:

- `open test/index.html` converts to `file://...`.
- bare domains like `example.com` become `https://example.com`.
- `tab-new <url>` uses the same URL normalization helper.

Still to keep consistent:

- Any future navigation-like command should use the same helper.

Relevant helper:

- `normalizeTargetURL` in `cmd/commands/core.go`

### 6. Make `run-code` Honest or Useful

Current state:

- `run-code` is not daemon-backed.
- It calls `page.Evaluate(code, nil)`.
- It does not support the documented `async page => { ... }` style.

Options:

1. Rename docs toward `eval` and leave `run-code` as local-only.
2. Implement daemon-backed `run-code` as expression evaluation.
3. Implement a small JS wrapper that accepts `async page => { ... }` semantics if feasible in Go/Playwright.

## Testing Plan for Next Work

Before editing:

```bash
go test ./...
go vet ./...
go build -o go-browser .
go-browser status
```

After each daemon feature:

```bash
go-browser stop
go-browser start
go-browser open 'data:text/html,<title>Smoke</title><button id="btn">Click</button><input id="name">'
go-browser <new-command>
go-browser stop
```

### Keyboard and Mouse

```bash
go-browser start
go-browser open /tmp/go-browser-input-smoke.html
go-browser click "#name"
go-browser type "abc"
go-browser press Backspace
go-browser keydown Shift
go-browser keyup Shift
go-browser mousemove 20 30
go-browser mousedown left
go-browser mouseup left
go-browser mousewheel 0 120
go-browser stop
```

### Tabs

```bash
go-browser start
go-browser open 'data:text/html,<title>One</title><h1>one</h1>'
go-browser tab-new 'data:text/html,<title>Two</title><h1>two</h1>'
go-browser tab-list
go-browser tab-select 0
go-browser eval 'document.title'
go-browser tab-close 1
go-browser stop
```

Recommended smoke groups:

### Core

```bash
go-browser start
go-browser open https://example.com
go-browser snapshot
go-browser goto https://httpbin.org
go-browser go-back
go-browser go-forward
go-browser reload
go-browser stop
```

### Elements

```bash
go-browser start
go-browser open 'data:text/html,<button id="btn" onclick="document.body.dataset.clicked=1">Click</button><input id="name"><div id="hover" style="width:100px;height:30px">Hover</div>'
go-browser click "#btn"
go-browser fill "#name" "abc"
go-browser hover "#hover"
go-browser eval "document.body.dataset.clicked"
go-browser stop
```

### Refs, Once Implemented

```bash
go-browser start
go-browser open 'data:text/html,<button id="btn">Click</button><input id="name">'
go-browser snapshot
go-browser click e0
go-browser fill e1 abc
go-browser stop
```

### Help Surface

```bash
go-browser --help
for c in open attach eval snapshot route run-code video-chapter cookie-set; do
  go-browser "$c" --help
done
```

## Documentation Notes

The current docs in `docs/go-browser` were revised to match the current implementation. If implementing any item above, update:

- `docs/go-browser/SKILL.md`
- The relevant file under `docs/go-browser/references/`

Avoid documenting a command as fully usable until it has daemon-backed smoke coverage.

## Generated / Local Artifacts

The workspace currently contains generated binaries:

```text
go-browser
go-browser.exe
go-browser.exe~
```

This directory is not a Git repository in the current environment, so no `.gitignore` status was verified. If this becomes a repo, add build outputs to `.gitignore` or move them under a dedicated ignored `bin/` directory.
