# Specification: HMAC Webhook Outbound Signature Signing & Anti-Spoofing

## 1. Overview
When WhatsADK successfully verifies an inbound HMAC verification token (`VERIFY <app_id>:<token>:<timestamp>:<signature>`), it delivers an HTTP POST notification to the tenant app's configured `webhook_url`. Currently, the request lacks cryptographic signing, which leaves tenant webhooks vulnerable to spoofing if an attacker probes the webhook endpoint directly.

This specification introduces outbound signature signing on outgoing HMAC webhooks using the shared tenant secret (`appCfg.GetSecret()`), preventing spoofing and replay attacks.

## 2. Requirements

### 2.1 Outbound Signature Header
When sending the POST request to `webhook_url`:
1. Calculate a current Unix timestamp in seconds (`timestampNow := time.Now().Unix()`).
2. Include the timestamp as header `X-Timestamp: <timestampNow>`.
3. Compute the outbound signature over the canonical payload:
   - Option / Standard: HMAC-SHA256 of `fmt.Sprintf("%d:%s", timestampNow, string(payloadBytes))` (or HMAC of payload body directly, standard GitHub/Stripe style: `HMAC-SHA256(secret, payloadBytes)` with `X-Signature-256: sha256=<hex>` and `X-Signature-Timestamp: <timestamp>`).
   - We will sign `fmt.Sprintf("%d:%s", timestampNow, string(payloadBytes))` so the signature commits to both the timestamp and the entire JSON payload body.
4. Set headers:
   - `Content-Type: application/json`
   - `X-Signature-256: sha256=<hex_digest>`
   - `X-Signature-Timestamp: <unix_timestamp>`

### 2.2 Security Properties
- **Anti-Spoofing:** Third-party tenant webhooks can verify that only WhatsADK (possessing the shared secret) originated the webhook.
- **Anti-Replay:** Tenants can reject requests where `abs(now - X-Signature-Timestamp) > tolerance` (e.g. 5 minutes).
- **Constant-Time Verification Helper:** Provide a public/internal helper function `VerifyWebhookSignature(secret, body, timestamp, signature string, tolerance time.Duration) error` to facilitate testing and tenant reference implementations.

### 2.3 Backward Compatibility & Error Handling
- Existing HMAC apps will immediately benefit from signed headers without changes to `config.yaml`.
- The webhook payload structure (`{"from": "...", "body": "..."}`) remains unchanged.
- Tests will verify signature generation, header presence, and signature verification helpers.
