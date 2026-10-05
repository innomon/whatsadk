# Implementation Plan: Cron Heartbeat Timers

## Goal
Implement a configurable cron-based heartbeat system that executes A2A agent tasks and maintains a summary memory across runs.

## Checklist

### 1. Configuration Update
- [ ] Add `CronConfig` and `CronJobConfig` structs to `internal/config/config.go`.
- [ ] Update `Config` struct to include `Cron CronConfig`.
- [ ] Implement defaults and environment variable overrides for cron settings.

### 2. Memory Persistence
- [ ] Implement `internal/cron/store.go` to use `internal/store.Store.PutFile` and `GetFile`.
- [ ] Use paths like `cron/<job_name>/summary` to store and retrieve summaries.
- [ ] Ensure the `filesys` table is available in the configured database.

### 3. Cron Manager
- [ ] Create `internal/cron/manager.go`.
- [ ] Integrate `github.com/robfig/cron/v3` for scheduling.
- [ ] Implement `Job` execution logic:
    - [ ] Load last summary from store.
    - [ ] Initialize ADK client for the job.
    - [ ] Send message to agent (including previous summary if available).
    - [ ] Capture response and generate a summary.
    - [ ] Save new summary to store.
    - [ ] Log outcome.

### 4. Integration
- [ ] Update `cmd/gateway/main.go` to initialize and start the `CronManager` if enabled.
- [ ] Ensure proper shutdown of cron jobs on gateway exit.

### 5. Testing & Validation
- [ ] Add unit tests for `CronManager` scheduling.
- [ ] Create a mock ADK server to test heartbeat execution.
- [ ] Verify that summaries are correctly persisted and passed to subsequent runs.

## Technical Details

### Memory Injection
The summary from the previous run can be passed as a "system" part or prepended to the user message to the ADK agent.
Example:
`Previous Summary: [Summary from T-1]. Current Task: [Message from Config]`

### Summary Extraction
The `extractFinalParts` logic in `internal/agent/client.go` can be reused. We might want the agent to explicitly provide a "SUMMARY" part or just use the whole response as the summary for the next run.
