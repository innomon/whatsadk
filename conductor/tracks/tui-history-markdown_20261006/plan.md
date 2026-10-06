# Implementation Plan: TUI Command History & Markdown Rendering (`tui-history-markdown_20261006`)

## Phase 1: Command History Navigation
- [ ] Task: Add `cmdHistory []string` and `historyIdx int` state to `AppModel` in `internal/tui/app.go`.
- [ ] Task: Intercept `up` and `down` key presses in `StateNormal` to cycle through command history and restore draft text.

## Phase 2: Glamour Markdown Renderer Integration
- [ ] Task: Add Glamour (`github.com/charmbracelet/glamour`) markdown renderer integration in `internal/tui/app.go`.
- [ ] Task: Render `ResultTypeMarkdown` contents using Glamour dark style renderer before appending to viewport content.

## Phase 3: Layout & Prompt Cleanup
- [ ] Task: Refine prompt line and status bar formatting to remove stray arrows or layout overlapping.

## Phase 4: Verification & Packaging
- [ ] Task: Add unit tests for history navigation and renderer in `internal/tui/app_test.go`.
- [ ] Task: Run `go test ./...` and rebuild binaries using `make build`.
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Verification & Packaging'
