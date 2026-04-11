# GEMINI.md - WhatsADK Project Mandates

This file contains foundational mandates for Gemini CLI when working on the **WhatsADK** project. These instructions take precedence over general workflows.

## 🎯 Core Objectives
- Maintain a secure, high-performance Go gateway between WhatsApp and ADK services.
- Ensure all changes are verified with tests and follow the established security model (JWT/EdDSA).
- **CRITICAL**: Always update `README.md` after making any code changes that affect functionality, configuration, or architecture.

## 🛠 Engineering Standards

### Go Development
- **Style**: Adhere strictly to [Effective Go](https://go.dev/doc/effective_go) and Go Code Review Comments.
- **Formatting**: Always use `gofmt` and `goimports`.
- **Error Handling**: Never ignore errors with `_`. Handle them explicitly and return early to reduce nesting.
- **Testing**: Use table-driven tests with `t.Run()`. Tests must reside in the same package as the code they test.
- **Concurrency**: Use `context.Context` as the first parameter for functions involving cancellation or timeouts.
- use on postgres database, **DO NOT** use sqilite for this project.
- never use the spf13 library (Cobra/Pflag). Instead, always implement a handcrafted command registry for CLI and slash commands.


### Architecture & Patterns
- **Directory Structure**:
    - `cmd/`: Application entry points.
    - `internal/`: Private application code.
    - `pkg/`: Public library code (if applicable).
- **Configuration**: Follow the search order: CLI flag `-config` > `CONFIG_FILE` env var > `./config.yaml` > `./config/config.yaml`.
- **Dependencies**: Use `whatsmeow` for WhatsApp and `adk-go` for ADK interactions. Do not introduce new DI frameworks; use manual dependency injection.

### Security
- **JWT**: RS256 for standard ADK communication.
- **OAuth**: Ed25519/EdDSA for WhatsApp-based login.
- **Secrets**: Never hardcode keys or tokens. Use `config.yaml` or environment variables.

## 🚀 Workflow Mandates
1. **Verification**: Every logic change MUST be accompanied by a test case or a reproduction script.
2. **Build**: Run `go build ./...` to ensure no regressions.
3. **Lint**: Run `golangci-lint run` if available.
4. **Documentation**: Keep `ARCHITECTURE.md` and `README.md` in sync with the codebase.

## 📚 References
- WhatsApp Client: [whatsmeow](https://github.com/tulir/whatsmeow)
- ADK Go SDK: [adk-go](https://github.com/google/adk-go)
