# Implementation Plan: Reverse ADK Simulator

## Phase 1: Foundation
- [ ] Create directory `internal/adksim`.
- [ ] Define shared types in `internal/adksim/types.go`:
    - `IncomingRequest`: Wraps `agent.RunRequest` with a unique ID and response channel.
    - `OutgoingResponse`: Wraps text and media attachments.
- [ ] Create `cmd/adksim/main.go` entry point with basic flag parsing (port, appName).

## Phase 2: HTTP Server logic
- [ ] Implement `Server` struct in `internal/adksim/server.go`.
- [ ] Implement HTTP handlers for `/run` and `/run_sse`.
- [ ] Use a registry (map) to track pending requests and their response channels.
- [ ] Implement media extraction logic to save incoming attachments to disk.

## Phase 3: TUI Development (Bubble Tea)
- [ ] Implement `Model` in `internal/adksim/tui.go`.
- [ ] Set up `viewport` for history and `textarea` for input.
- [ ] Handle `IncomingRequest` messages to update the UI.
- [ ] Implement command parsing (e.g., `/attach`).
- [ ] Implement "Send" logic that fulfills the oldest pending HTTP request.

## Phase 4: Integration & Verification
- [ ] Update `Makefile` to build `adksim`.
- [ ] Manual Test 1: Simple text exchange between `simulator` -> `gateway` -> `adksim`.
- [ ] Manual Test 2: Send image from `simulator` -> `adksim` and verify it's saved.
- [ ] Manual Test 3: Send document from `adksim` -> `simulator` and verify it's received.
- [ ] Verify SSE streaming (if applicable).

## Phase 5: Documentation
- [ ] Update `README.md` with instructions on how to use `adksim`.
- [ ] Update `ARCHITECTURE.md` to include the reverse simulator in the testing section.
