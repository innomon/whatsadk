# Implementation Plan: Multi-Tenant App Verification Routing with HMAC Validation & Rate Limiting

## Phase 1: Configuration & App Registry
- [x] Extend `VerificationConfig` in `internal/config/config.go` to support multi-tenant HMAC app definitions (`auth_type: "hmac"`, `secret`, `webhook_url`, `ttl`, `success_message`) alongside existing RSA apps.
- [x] Implement sender rate-limiter utility in `internal/verification/ratelimit.go` (sliding window / bucket per sender phone).
- [x] Add unit tests in `internal/config/config_test.go` and `internal/verification/ratelimit_test.go`.

## Phase 2: HMAC Token Parser & Verification Engine
- [x] Create `internal/verification/hmac.go` with parsing and constant-time HMAC validation helpers (`ParseHMACVerificationToken`, `VerifyHMAC`).
- [x] Update `internal/verification/handler.go` to support both HMAC tokens and legacy RSA JWT tokens.
- [x] Add unit tests for HMAC token parsing, invalid format, timestamp expiration, signature mismatch, and valid tokens.

## Phase 3: Gateway Message Pipeline Integration
- [x] In `internal/whatsapp/client.go#handleMessage`:
  - Intercept messages matching `VERIFY ` before whitelist/ADK dispatch.
  - Route through verification handler.
  - On valid verification: dispatch HTTP POST webhook to target app, send WhatsApp confirmation reply, record audit record in `filesys`, and return early without calling ADK agent.
- [x] Test end-to-end flow with simulated webhook receiver and live WhatsApp message event.

## Phase 4: Staging Configuration & Deployment
- [x] Update staging config `config_num_425.yaml` with the `kb` app verification section and webhook URL (`http://localhost:5001/api/auth/whatsapp/webhook`).
- [x] Rebuild and verify `./gateway -config config_num_425.yaml`.
