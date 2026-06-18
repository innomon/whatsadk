# Specification: Cron Heartbeat Timers

## Overview
Implement an optional cron-like heartbeat timer system for the WhatsADK gateway. This system allows the gateway to periodically execute A2A (Agent-to-Agent) tasks on remote ADK servers based on a configurable schedule.

## Requirements

### 1. Configuration
- The `config.yaml` file will include an optional `cron` section.
- If the `cron` section is missing or empty, no heartbeat timers will run.
- Each cron entry will specify:
    - `name`: A unique identifier for the cron job.
    - `schedule`: A cron-style schedule string (e.g., `0 */5 * * * *` for every 5 minutes).
    - `agent`: The A2A agent configuration (endpoint, app name, etc., or use defaults).
    - `user_id`: The user ID to use for the agent session.
    - `message`: The message/command to send to the agent.

### 2. A2A Agent Execution
- For each scheduled run, the gateway will call the configured remote ADK agent.
- The execution will use the existing `internal/agent/Client` logic.
- The "memory" from the previous run (a summary) must be included in the request.

### 3. Memory & Summary
- After each run, a summary of the outcome must be saved.
- This summary will be provided to the agent in the *next* scheduled run to provide continuity.
- Memory will be persisted in the `filesys` table in the database (using the existing `internal/store` logic).
- Path pattern: `cron/<job_name>/summary`.
- `content`: The summary text.
- `metadata`: Execution metadata (e.g., job name, timestamp, success/failure).

### 4. Recording Outcomes
- The outcome of each cron execution (success/failure, response summary, timestamp) must be logged and optionally stored.

## Proposed Configuration Schema (YAML)
```yaml
cron:
  enabled: true
  jobs:
    - name: "health-check"
      schedule: "@every 5m"
      user_id: "system-heartbeat"
      message: "Perform system health check and provide a summary."
      agent:
        endpoint: "http://remote-adk:8000/api"
        app_name: "monitor-agent"
```

## Proposed Architecture
- `internal/cron`: A new package to manage the lifecycle of heartbeat timers.
- `internal/cron/manager.go`: Orchestrates the schedules and executes jobs.
- `internal/cron/store.go`: Handles persistence of summaries.
- `internal/config`: Update to include `CronConfig`.
