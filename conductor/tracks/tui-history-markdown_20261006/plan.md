# Implementation Plan: TUI Command History & Markdown Rendering (`tui-history-markdown_20261006`)

## Phase 1: Command History Navigation
- [x] Task: Add `cmdHistory []string` and `historyIdx int` state to `AppModel` in `internal/tui/app.go`.
- [x] Task: Intercept `up` and `down` key presses in `StateNormal` to cycle through command history and restore draft text.

## Phase 2: Glamour Markdown Renderer Integration
- [x] Task: Add Glamour (`github.com/charmbracelet/glamour`) markdown renderer integration in `internal/tui/app.go`.
- [x] Task: Render `ResultTypeMarkdown` contents using Glamour dark style renderer before appending to viewport content.

## Phase 3: Layout & Prompt Cleanup
- [x] Task: Refine prompt line and status bar formatting to remove stray arrows or layout overlapping.

## Phase 4: Verification & Packaging
- [x] Task: Add unit tests for history navigation and renderer in `internal/tui/app_test.go`.
- [x] Task: Run `go test ./...` and rebuild binaries using `make build`.
- [x] Task: Conductor - User Manual Verification 'Phase 4: Verification & Packaging'
