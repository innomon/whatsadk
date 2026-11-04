# Specification: Multi-Tenant App Verification Routing with HMAC Validation & Rate Limiting (Approach B)

## 1. Overview
This track introduces a lightweight, multi-tenant reverse-OTP verification engine into the WhatsADK gateway. It intercepts compact namespaced HMAC tokens (`VERIFY <app_id>:<token>:<timestamp>:<signature>`), validates the signature and timestamp in-memory, applies sender rate limiting, and forwards verified identity payloads to registered backend webhooks without exposing web apps to spam or collision.

## 2. Problem Statement
1. **Multi-Tenant Ambiguity**: In shared or multi-instance gateway deployments, raw tokens (`VERIFY <hex>`) cannot identify which backend web app initiated the challenge.
2. **Spam & Webhook Flooding**: Unauthenticated or forged messages arriving on WhatsApp could trigger indiscriminate outbound HTTP webhook calls, causing Denial-of-Service (DoS) and connection exhaustion on downstream application servers.
3. **Heavyweight JWT Overhead**: Full RS256 JWTs (`eyJ...`) create bloated WhatsApp messages (~400 characters) that are cumbersome for users and mobile SMS/WhatsApp previews.

## 3. Protocol & Format Contract
```
VERIFY <app_id>:<token>:<timestamp>:<signature>
```

- **Prefix**: `VERIFY` (case-insensitive command trigger).
- **App ID (`app_id`)**: Alphanumeric tenant identifier (e.g., `kb` for Kheti-Badi, `orez` for Orez Laundry).
- **Token**: Application session hex token (e.g., 32 hex chars).
- **Timestamp**: Unix epoch timestamp in seconds.
- **Signature**: Hex-encoded HMAC-SHA256 computed over `<app_id>:<token>:<timestamp>` using the app's shared secret.

## 4. Gateway Verification Pipeline

```
Incoming WhatsApp Message
       │
       ▼
Starts with "VERIFY "? ──no──► Normal ADK Agent Pipeline
       │ yes
       ▼
Check Sender Rate Limiter (e.g., max 5/min) ──exceeded──► Warn & Drop
       │ allowed
       ▼
Extract <app_id> & Lookup in Config Apps
       │ not found
       ▼
Log Warning & Drop (Do NOT call any backend)
       │ found
       ▼
Validate Timestamp Freshness (|now - timestamp| <= App TTL)
       │ expired
       ▼
Drop / Reply Expired
       │ valid
       ▼
Verify HMAC-SHA256 Signature (Constant-Time)
       │ invalid / forged
       ▼
Log Tamper Warning & Drop
       │ valid
       ▼
Dispatch HTTP POST to App Webhook:
  POST <app.webhook_url>
  Content-Type: application/json
  {
    "from": "<sender_phone>",
    "body": "VERIFY <app_id>:<token>:<timestamp>:<signature>"
  }
       │
       ▼
Log Audit to filesys & Reply Confirmation via WhatsApp
(e.g., "✅ Verification successful! Please return to Kheti-Badi.")
       │
       ▼
Return (Do NOT forward to ADK agent)
```

## 5. Configuration Schema

Extend `verification` in `config/config.yaml` / `config_num_*.yaml`:

```yaml
verification:
  enabled: true
  rate_limit:
    max_attempts: 5
    window: "1m"
  apps:
    kb:
      name: "kheti-badi"
      auth_type: "hmac"
      secret: "${KHETI_BADI_HMAC_SECRET}"
      webhook_url: "http://localhost:5001/api/auth/whatsapp/webhook"
      ttl: "5m"
      success_message: "✅ Verified successfully! Please return to Kheti-Badi."
```

## 6. Security Considerations
- **In-Memory Replay & DoS Defense**: Expired timestamps and invalid signatures are dropped before creating an HTTP client connection or executing any network I/O.
- **Constant-Time Verification**: `hmac.Equal` prevents timing-based signature recovery attacks.
- **Per-Sender Rate Limiting**: Prevents automated bot swarms from repeatedly invoking cryptographic validation on the gateway.
