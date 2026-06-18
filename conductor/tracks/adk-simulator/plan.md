# Implementation Plan: Reverse ADK Simulator

## Phase 1: Foundation
- [x] Create directory `internal/adksim`.
- [x] Define shared types in `internal/adksim/types.go`:
    - `IncomingRequest`: Wraps `agent.RunRequest` with a unique ID and response channel.
    - `OutgoingResponse`: Wraps text and media attachments.
- [x] Create `cmd/adksim/main.go` entry point with basic flag parsing (port, appName).

## Phase 2: HTTP Server logic
- [x] Implement `Server` struct in `internal/adksim/server.go`.
- [x] Implement HTTP handlers for `/run` and `/run_sse`.
- [x] Use a registry (channel-based) to track pending requests and their response channels.
- [x] Implement media extraction logic to save incoming attachments to disk.

## Phase 3: TUI Development (Bubble Tea)
- [x] Implement `Model` in `internal/adksim/tui.go`.
- [x] Set up `viewport` for history and `textarea` for input.
- [x] Handle `IncomingRequest` messages to update the UI.
- [x] Implement command parsing (e.g., `/attach`).
- [x] Implement "Send" logic that fulfills the oldest pending HTTP request.

## Phase 4: Integration & Verification
- [x] Update `Makefile` to build `adksim`.
- [ ] Manual Test 1: Simple text exchange between `simulator` -> `gateway` -> `adksim`.
- [ ] Manual Test 2: Send image from `simulator` -> `adksim` and verify it's saved.
- [ ] Manual Test 3: Send document from `adksim` -> `simulator` and verify it's received.
- [x] Verify SSE streaming (Mocked in ADKSim to return full response via SSE).

## Phase 5: Documentation
- [x] Update `README.md` with instructions on how to use `adksim`.
- [x] Update `ARCHITECTURE.md` to include the reverse simulator in the testing section.
