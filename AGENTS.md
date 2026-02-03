# AGENTS.md

## Build/Test Commands
- Build: `go build ./...`
- Test all: `go test ./...`
- Test single: `go test -run TestName ./path/to/package`
- Test verbose: `go test -v ./...`
- Lint: `golangci-lint run`
- Format: `gofmt -w .` or `goimports -w .`

## Architecture
- cmd/ - Application entrypoints
- internal/ - Private application code
- pkg/ - Public library code
- *_test.go - Test files (same package)

## Code Style
- Follow [Effective Go](https://go.dev/doc/effective_go) and Go Code Review Comments
- Use `gofmt`/`goimports` for formatting
- Handle errors explicitly; never ignore with `_`
- Prefer short variable names in small scopes (e.g., `r`, `w`, `ctx`)
- Use table-driven tests with `t.Run()` for subtests
- Return early to reduce nesting; avoid else after return
- Use context.Context as first parameter for cancellation/timeouts

## Config file
All configuration will be through a `config.yaml` file, 1st check if it is passed via arg, if not present
check the env var CONFIG_FILE, if not present search for the config file in curerent dir next in `config` sub dir, if not in the same dir as the executable is running from, or in the `config` sub dir of the executable path. 

## Code Reference
- Use [whatsapp go client](https://github.com/tulir/whatsmeow) to connect to whatsapp
- Use [ADK go SDK](https://github.com/google/adk-go) to connect with agents
