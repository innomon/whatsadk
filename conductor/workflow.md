# Development Workflow - WhatsADK

## Operational Principles
1. **Test-Driven & Verification First:**
   - Every feature or logic modification MUST be verified with Go unit tests (`go test ./...`).
   - Every binary build must compile cleanly native (`make build`) and cross-platform (`make build-cross`).
2. **Handcrafted Registries Mandate:**
   - Never introduce `cobra` or `pflag` (`spf13`). Always implement handcrafted command registries.
3. **Workspace-Root Relative Paths:**
   - Use clean workspace-root relative paths in documentation and scripts.
4. **Documentation Updates:**
   - Always update `README.md` and relevant agent guides (e.g. `cmd/mcp/AGENT.md`) after completing feature modifications.
5. **Checkpointing & Verification Protocol:**
   - Each phase in track implementation plans includes manual user verification and checkpointing before marking completed.
