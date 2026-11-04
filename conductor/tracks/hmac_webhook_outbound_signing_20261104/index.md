# Track: HMAC Webhook Outbound Signature Signing & Anti-Spoofing

- **Track ID**: `hmac_webhook_outbound_signing_20261104`
- **Specification**: [spec.md](spec.md)
- **Plan**: [plan.md](plan.md)
- **Metadata**: [metadata.json](metadata.json)

## Scope
Cryptographically sign outbound HTTP webhook POST requests sent to tenant `webhook_url` during HMAC reverse-OTP verification using `HMAC-SHA256(secret, timestamp + ":" + body)`. Attach `X-Signature-256` and `X-Signature-Timestamp` headers to prevent webhook spoofing and replay attacks.
