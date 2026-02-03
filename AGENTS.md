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
