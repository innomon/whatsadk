# Implementation Plan: In-Process ADK Runner in Router Example

This plan outlines the steps to add in-process execution of ADK agents to the `examples/router` application. This allows routing to agents defined by `agentic` configuration files without requiring a separate network hop.

## User Requirements
- Support an array of `AppIds` associated with `agentic` config files.
- Store `runner.Runner` instances in a map keyed by `AppId`.
- Instantiate agents in-process and route requests to them.

## Proposed Changes

### 1. `go.mod`
- Add `github.com/innomon/agentic` dependency.
- Ensure `google.golang.org/adk` is up to date.

### 2. `examples/router/main.go`
- Update `AppConfig` struct to include `AgenticConfig` (path to agentic config file).
- Implement a `RunnerManager` struct to manage in-process runners:
  ```go
  type RunnerManager struct {
      runners map[string]*runner.Runner
      sessionService session.Service
  }
  ```
- Add initialization logic in `main()` to load in-process agents.
- Update `routerRun` to check for an in-process runner before falling back to `agent.NewClient`.

### 3. `examples/router/router.yaml`
- Add examples of apps using `agenticConfig`.

## Checkpoints

### Checkpoint 1: Infrastructure & Config
- [x] Add `github.com/innomon/agentic` to `go.mod`.
- [x] Update `AppConfig` struct in `examples/router/main.go`.
- [x] Update `router.yaml` with test configuration.

### Checkpoint 2: Runner Manager Implementation
- [x] Implement `NewRunnerManager` and its initialization logic.
- [x] Integrate `RunnerManager` into the `main` function.

### Checkpoint 3: Routing Logic Integration
- [x] Modify `routerRun` to use `RunnerManager`.
- [x] Handle streaming/events from `runner.Runner` and convert them to `session.Event`.

### Checkpoint 4: Verification
- [x] Create a mock agentic config.
- [x] Test routing to the in-process agent.
- [x] Test fallback to remote agents.

## Technical Details

### Agentic Initialization
```go
cfg, err := agenticconfig.Load(path)
reg := registry.New(cfg)
lc, err := reg.BuildLauncherConfig(ctx)
ag, err := registry.Get[agent.Agent](ctx, reg, agentName)
r, err := runner.New(runner.Config{
    AppName:        agentName,
    Agent:          ag,
    SessionService: lc.SessionService,
})
```

### Routing
```go
if r, ok := runnerManager.Get(targetApp); ok {
    // Call r.Run(...)
} else {
    // Fallback to HTTP client
}
```
