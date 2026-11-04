# Implementation Plan: Verification Database Auto-Whitelisting & Blacklist Explicit Rejection

- [x] Phase 1: Store Whitelist Interface & Backend Implementations <!-- id: phase1 -->
  - [x] Define `WhitelistedNumber` struct and interface methods on `StoreBackend` in `internal/store/store.go` <!-- id: 1.1 -->
  - [x] Implement SQL schema and CRUD for PostgreSQL/SQLite in `internal/store/store.go` <!-- id: 1.2 -->
  - [x] Implement SQLite-P2P crm_store view and methods in `internal/store/p2p_backend.go` and `internal/store/p2p_whitelist.go` <!-- id: 1.3 -->
  - [x] Implement SurrealDB methods in `internal/store/surreal.go` <!-- id: 1.4 -->
  - [x] Add unit tests for `whitelisted_numbers` in `internal/store/store_test.go` and `internal/store/p2p_backend_test.go` <!-- id: 1.5 -->

- [x] Phase 2: Gateway Pipeline Verification & Access Control Updates <!-- id: phase2 -->
  - [x] Update `internal/whatsapp/client.go#handleMessage` to explicitly reply with `messages.Blacklisted` when a blacklisted number attempts verification <!-- id: 2.1 -->
  - [x] Update `internal/whatsapp/client.go#handleMessage` to auto-whitelist verified phone numbers in `c.store` upon successful verification <!-- id: 2.2 -->
  - [x] Update `internal/whatsapp/client.go#isUserAllowed` to dynamically check `c.store.IsWhitelisted(ctx, phone)` <!-- id: 2.3 -->
  - [x] Add unit tests in `internal/whatsapp/client_test.go` testing blacklist rejection message and auto-whitelisting <!-- id: 2.4 -->

- [x] Phase 3: Verification & Documentation <!-- id: phase3 -->
  - [x] Run full test suite `go test ./...` and `go build ./...` <!-- id: 3.1 -->
  - [x] Update `README.md` and `ARCHITECTURE.md` with auto-whitelisting and blacklist rejection behavior <!-- id: 3.2 -->
  - [x] Mark Conductor track complete and archive <!-- id: 3.3 -->
