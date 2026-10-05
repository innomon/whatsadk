# Implementation Plan: Structured Logging and Log Rotation

This plan outlines the steps to introduce structured logging (human-readable console + caller-identifying JSONL file) and log file rotation to the WhatsADK project.

## Phase 1: Configuration & Types
- [ ] Add `LoggingConfig` struct in `internal/config/config.go` and map it to YAML/Env vars.
- [ ] Define default logging options (level = "INFO", dir = "logs", file_name = "whatsadk.log", max_size_mb = 10, max_backups = 5).

## Phase 2: Log Rotation Engine
- [ ] Implement `LogRotator` in `internal/logger/rotator.go` implementing `io.Writer`.
- [ ] Write unit tests for `LogRotator` verifying rotation triggers and backups pruning.

## Phase 3: Central Logger Initialization
- [ ] Implement `Init(cfg *config.Config)` in `internal/logger/logger.go` utilizing `slog.New` with:
  - `slog.NewTextHandler(os.Stdout, ...)` for console.
  - `slog.NewJSONHandler(rotator, &slog.HandlerOptions{AddSource: true, ...})` for file.
- [ ] Implement custom `waLog.Logger` wrapper (`NewWhatsMeowLogger(s *slog.Logger)`) for compatibility with `whatsmeow`.

## Phase 4: Integration
- [ ] Update `cmd/gateway/main.go` and `cmd/waba-gateway/main.go` to initialize the global logger.
- [ ] Update `internal/whatsapp/client.go` to use the wrapped `whatsmeow` logger.
- [ ] Update `internal/verification/handler.go` to accept and use the unified structured logger.

## Phase 5: Verification & Verification tests
- [ ] Run `go build ./...` to verify compilation.
- [ ] Run `go test ./...` to ensure no regressions.
- [ ] Verify logs are written to the configured directory in JSONL format, containing caller source file location information.
