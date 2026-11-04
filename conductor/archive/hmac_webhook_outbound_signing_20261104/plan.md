# Implementation Plan: HMAC Webhook Outbound Signature Signing

- [x] Phase 1: Cryptographic Signing & Verification Utilities <!-- id: phase1 -->
  - [x] Implement outbound signature generation `SignWebhookPayload(secret string, timestamp int64, body []byte) string` in `internal/verification/hmac.go` <!-- id: 1.1 -->
  - [x] Implement outbound signature verification helper `VerifyWebhookSignature(secret string, body []byte, timestampHeader, signatureHeader string, tolerance time.Duration) error` <!-- id: 1.2 -->
  - [x] Add unit tests for signature generation, timestamp tampering, expired timestamps, and body alteration in `internal/verification/handler_test.go` <!-- id: 1.3 -->

- [x] Phase 2: Gateway Handler Integration <!-- id: phase2 -->
  - [x] Update `postHMACWebhook` in `internal/verification/handler.go` to accept the tenant secret and attach `X-Signature-256` and `X-Signature-Timestamp` headers <!-- id: 2.1 -->
  - [x] Update existing tests in `internal/verification/handler_test.go` and `internal/whatsapp/client_test.go` to assert and validate headers <!-- id: 2.2 -->
  - [x] Run full test suite `go test ./...` and `go build ./...` <!-- id: 2.3 -->

- [x] Phase 3: Documentation & Tenant Verification Reference <!-- id: phase3 -->
  - [x] Update `README.md` and `ARCHITECTURE.md` documenting webhook signature verification with code snippets <!-- id: 3.1 -->
  - [x] Update staging config or documentation if applicable <!-- id: 3.2 -->
  - [x] Mark Conductor track complete and prepare archive <!-- id: 3.3 -->
