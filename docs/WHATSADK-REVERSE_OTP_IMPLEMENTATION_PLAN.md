# WhatsADK — Reverse OTP Verification Implementation Plan

> Implementation plan for adding Reverse OTP (Mobile-Originated Verification) support to the [WhatsADK gateway](https://github.com/innomon/whatsadk).

---

## 1. Overview

The WhatsADK gateway currently bridges WhatsApp messages to Google ADK agents. This plan adds a **verification message handler** that intercepts specially formatted JWT messages from users, validates them, and calls back the originating application's backend to confirm phone number ownership.

This is the gateway-side counterpart to the backend flow documented in [Backend auth spec](https://github.com/innomon/orez-laundry-app/blob/main/docs/otp-logic.md).

---

## 2. Current Architecture

```
┌──────────────┐     whatsmeow      ┌──────────────────┐      HTTP       ┌─────────────┐
│   WhatsApp   │ ◄─────────────────► │  WhatsADK GW     │ ──────────────► │  ADK Agent   │
│   (User)     │   bidirectional     │  cmd/gateway/     │   /run or      │  Service     │
└──────────────┘                     │  main.go          │   /run_sse     └─────────────┘
                                     ├──────────────────┤
                                     │ internal/         │
                                     │   auth/           │ ← RS256 JWT gen
                                     │   config/         │ ← YAML + env
                                     │   agent/          │ ← ADK HTTP client
                                     │   whatsapp/       │ ← whatsmeow wrapper
                                     └──────────────────┘
```

### Current Message Flow

1. User sends WhatsApp message → `whatsmeow` event handler receives it
2. Gateway generates an RS256 JWT with `user_id` = sender phone, `channel` = "whatsapp"
3. Gateway calls ADK service (`/run` or `/run_sse`) with the JWT in `Authorization` header
4. ADK response is sent back to user via WhatsApp

### Current File Structure

```
whatsadk/
├── cmd/gateway/main.go
├── internal/
│   ├── auth/
│   │   ├── claims.go          # JWT claims: user_id, channel
│   │   ├── jwt.go             # RS256 JWTGenerator (sign with private key)
│   │   └── jwt_test.go
│   ├── config/
│   │   └── config.go          # YAML + env config loader
│   ├── agent/
│   │   └── client.go          # ADK REST/SSE client
│   └── whatsapp/
│       └── client.go          # WhatsApp client (QR auth, message handling)
├── config/
│   └── config.yaml            # Default config
└── go.mod
```

---

## 3. Design: Verification Message Handler

### 3.1 Detection Strategy

When the gateway receives a WhatsApp message, it must determine whether the message is:

- **A verification token** — a JWT string (starts with `eyJ`) that should be validated and trigger a callback
- **A normal message** — forwarded to the ADK agent as usual

**Detection logic:**

```
incoming message
    │
    ▼
is message body a valid JWT?  ──no──► forward to ADK agent (existing flow)
    │ yes
    ▼
does JWT contain `callback_url` + `mobile` claims?  ──no──► forward to ADK agent
    │ yes
    ▼
verification flow
```

The JWT is parsed without signature verification first (header-only or claims peek) to check for verification-specific claims. Full RS256 signature verification happens only after identifying it as a verification token.

### 3.2 Verification Flow

```
┌──────────┐        ┌───────────────┐        ┌──────────────┐
│ WhatsApp │        │  WhatsADK GW  │        │ App Backend  │
│ (User)   │        │               │        │ (Kln API)    │
└────┬─────┘        └──────┬────────┘        └──────┬───────┘
     │ Send JWT message     │                        │
     ├─────────────────────►│                        │
     │                      │ 1. Detect JWT          │
     │                      │ 2. Parse claims        │
     │                      │ 3. Load app public key │
     │                      │ 4. Verify RS256 sig    │
     │                      │ 5. Check exp           │
     │                      │ 6. Match sender phone  │
     │                      │    vs claim.mobile     │
     │                      │                        │
     │                      │ 7. Sign callback JWT   │
     │                      │    (GW private key)    │
     │                      │                        │
     │                      │ POST callback_url      │
     │                      ├───────────────────────►│
     │                      │                        │ Verify GW JWT
     │                      │                        │ Mark challenge verified
     │                      │◄───────────────────────┤
     │                      │ 200 OK                 │
     │                      │                        │
     │ ✅ "Verified! You    │                        │
     │    can return to     │                        │
     │    the app."         │                        │
     │◄─────────────────────┤                        │
     │                      │                        │
     │  ── OR on failure ── │                        │
     │                      │                        │
     │ ❌ "Verification     │                        │
     │    failed: <reason>" │                        │
     │◄─────────────────────┤                        │
```

---

## 4. Specification

### 4.1 Incoming Verification JWT (from app backend → user → gateway)

**Algorithm:** RS256
**Signed by:** App backend's RSA private key
**Verified by:** Gateway using app's public key

**Claims:**

| Claim | Type | Required | Description |
|-------|------|----------|-------------|
| `mobile` | string | ✅ | User's phone number without `+` (e.g. `"910987654321"`) |
| `app_name` | string | ✅ | Application identifier, lowercase-with-dashes (e.g. `"orez-laundry-app"`) |
| `callback_url` | string | ✅ | Full URL the gateway should POST to after verification |
| `challenge_id` | string | ✅ | Unique challenge identifier (UUID) |
| `iat` | number | ✅ | Issued at (Unix timestamp) |
| `exp` | number | ✅ | Expiry (Unix timestamp) |

**Example decoded payload:**
```json
{
  "mobile": "910987654321",
  "app_name": "orez-laundry-app",
  "callback_url": "https://api.example.com/api/v1/auth/whatsapp/callback?challenge_id=abc-123",
  "challenge_id": "abc-123",
  "iat": 1739000000,
  "exp": 1739000300
}
```

### 4.2 Outgoing Callback JWT (gateway → app backend)

**Algorithm:** RS256
**Signed by:** Gateway's RSA private key
**Verified by:** App backend using gateway's public key

**Claims:**

| Claim | Type | Value |
|-------|------|-------|
| `user_id` | string | Sender's WhatsApp phone number without `+` (e.g. `"910987654321"`) |
| `channel` | string | `"whatsapp"` |
| `iss` | string | Gateway issuer (e.g. `"whatsadk-gateway"`) |
| `aud` | string | App name from the verification token (e.g. `"orez-laundry-app"`) |
| `iat` | number | Issued at (Unix timestamp) |
| `exp` | number | Expiry (Unix timestamp, short TTL — 2 minutes) |

**Example decoded payload:**
```json
{
  "user_id": "910987654321",
  "channel": "whatsapp",
  "iss": "whatsadk-gateway",
  "aud": "orez-laundry-app",
  "iat": 1739000010,
  "exp": 1739000130
}
```

### 4.3 Callback HTTP Request

```
POST <callback_url>
Authorization: Bearer <gateway_jwt>
Content-Type: application/json

(empty body)
```

The `callback_url` already contains the `challenge_id` as a query parameter. The gateway JWT in the `Authorization` header is all the app backend needs.

**Expected responses:**

| Status | Meaning | Gateway action |
|--------|---------|----------------|
| `200` | Verification accepted | Send success message to user |
| `400` | Challenge invalid/expired | Send failure message to user |
| `401` | JWT rejected | Send failure message to user |
| `5xx` | Server error | Send retry message or failure to user |

### 4.4 WhatsApp Response Messages

**Success:**
```
✅ Verification successful! You can now return to the app.
```

**Failure (invalid/expired token):**
```
❌ Verification failed. The link may have expired. Please request a new one from the app.
```

**Failure (phone mismatch):**
```
❌ Verification failed. Please make sure you're sending from the same number you registered with.
```

**Failure (server error):**
```
⚠️ Something went wrong. Please try again in a moment.
```

---

## 5. Configuration

### 5.1 New Config Fields

Add to `config/config.yaml`:

```yaml
verification:
  enabled: true
  callback_timeout: "10s"
  apps:
    orez-laundry-app:
      public_key_path: "secrets/apps/orez-laundry-app/public.pem"
    another-app:
      public_key_path: "secrets/apps/another-app/public.pem"
  messages:
    success: "✅ Verification successful! You can now return to the app."
    expired: "❌ Verification failed. The link may have expired. Please request a new one from the app."
    phone_mismatch: "❌ Verification failed. Please make sure you're sending from the same number you registered with."
    error: "⚠️ Something went wrong. Please try again in a moment."
```

**Design rationale — multi-app support:** The gateway may serve multiple applications simultaneously. Each app is identified by `app_name` in the verification JWT, and the gateway looks up the corresponding public key. This avoids deploying separate gateway instances per app.

### 5.2 Environment Variable Overrides

| Variable | Description |
|----------|-------------|
| `VERIFICATION_ENABLED` | `true`/`false` — master switch |
| `VERIFICATION_CALLBACK_TIMEOUT` | HTTP timeout for callback requests (default `10s`) |
| `VERIFICATION_APP_<NAME>_PUBLIC_KEY_PATH` | Public key path per app (e.g. `VERIFICATION_APP_OREZ_LAUNDRY_APP_PUBLIC_KEY_PATH`) |

### 5.3 Key File Organization

```
secrets/
├── jwt_private.pem              # Gateway's own private key (existing, for signing callback JWTs)
└── apps/
    ├── orez-laundry-app/
    │   └── public.pem           # orez-laundry-app's RS256 public key
    └── another-app/
        └── public.pem           # another-app's RS256 public key
```

---

## 6. Implementation Plan

### Phase 1: Verification Config + Key Registry

**File:** `internal/config/config.go`

Add verification configuration structs:

```go
type VerificationConfig struct {
    Enabled         bool                       `yaml:"enabled"`
    CallbackTimeout string                     `yaml:"callback_timeout"` // e.g. "10s"
    Apps            map[string]AppVerifyConfig  `yaml:"apps"`
    Messages        VerificationMessages        `yaml:"messages"`
}

type AppVerifyConfig struct {
    PublicKeyPath string `yaml:"public_key_path"`
}

type VerificationMessages struct {
    Success       string `yaml:"success"`
    Expired       string `yaml:"expired"`
    PhoneMismatch string `yaml:"phone_mismatch"`
    Error         string `yaml:"error"`
}
```

Add `Verification VerificationConfig` to the root `Config` struct.

**File:** `internal/auth/key_registry.go` (new)

```go
type KeyRegistry struct {
    appKeys map[string]*rsa.PublicKey  // app_name → public key
}

func NewKeyRegistry(apps map[string]AppVerifyConfig) (*KeyRegistry, error)
func (r *KeyRegistry) GetAppPublicKey(appName string) (*rsa.PublicKey, error)
```

Load all app public keys at startup. Return a clear error if any key file is missing or unparseable.

---

### Phase 2: Verification Token Parser

**File:** `internal/auth/verify_token.go` (new)

```go
// VerificationClaims represents the JWT claims from an app backend
type VerificationClaims struct {
    Mobile      string `json:"mobile"`
    AppName     string `json:"app_name"`
    CallbackURL string `json:"callback_url"`
    ChallengeID string `json:"challenge_id"`
    jwt.RegisteredClaims
}

// IsVerificationToken checks if a raw string looks like a verification JWT
// by peeking at the claims without verifying the signature.
// Returns the unverified claims if it looks like a verification token, nil otherwise.
func IsVerificationToken(raw string) *VerificationClaims

// VerifyVerificationToken fully verifies the JWT signature and claims
// using the app's public key from the registry.
func VerifyVerificationToken(raw string, appKey *rsa.PublicKey) (*VerificationClaims, error)
```

**`IsVerificationToken` logic:**
1. Try `jwt.Parser.ParseUnverified()` to extract claims
2. Check that `mobile`, `app_name`, and `callback_url` are all non-empty
3. If yes → return claims (for routing); if no → return nil

**`VerifyVerificationToken` logic:**
1. `jwt.ParseWithClaims()` with RS256 method enforcement
2. Validate `exp` is in the future
3. Validate `mobile`, `app_name`, `callback_url`, `challenge_id` are non-empty
4. Return verified claims

---

### Phase 3: Verification Handler

**File:** `internal/verification/handler.go` (new)

```go
type Handler struct {
    keys       *auth.KeyRegistry
    jwtGen     *auth.JWTGenerator  // existing — signs callback JWT
    httpClient *http.Client
    messages   VerificationMessages
    logger     *slog.Logger
}

func NewHandler(keys *auth.KeyRegistry, jwtGen *auth.JWTGenerator, cfg VerificationConfig, logger *slog.Logger) *Handler

// Handle processes a verification message.
// Returns the response message to send back to the user via WhatsApp.
func (h *Handler) Handle(ctx context.Context, senderPhone, messageBody string) string
```

**`Handle` method — step by step:**

```go
func (h *Handler) Handle(ctx context.Context, senderPhone, messageBody string) string {
    // 1. Parse unverified claims
    claims := auth.IsVerificationToken(messageBody)
    if claims == nil {
        return "" // not a verification token — caller should forward to ADK
    }

    // 2. Look up app public key
    appKey, err := h.keys.GetAppPublicKey(claims.AppName)
    if err != nil {
        h.logger.Warn("unknown app", "app_name", claims.AppName)
        return h.messages.Error
    }

    // 3. Full signature verification
    verified, err := auth.VerifyVerificationToken(messageBody, appKey)
    if err != nil {
        h.logger.Warn("verification token invalid", "error", err, "app", claims.AppName)
        return h.messages.Expired
    }

    // 4. Phone number match
    senderNormalized := stripPlus(senderPhone)
    if senderNormalized != verified.Mobile {
        h.logger.Warn("phone mismatch",
            "sender", senderNormalized,
            "claim_mobile", verified.Mobile,
        )
        return h.messages.PhoneMismatch
    }

    // 5. Sign callback JWT
    callbackJWT, err := h.jwtGen.TokenWithAudience(senderNormalized, verified.AppName)
    if err != nil {
        h.logger.Error("failed to sign callback JWT", "error", err)
        return h.messages.Error
    }

    // 6. POST to callback URL
    if err := h.postCallback(ctx, verified.CallbackURL, callbackJWT); err != nil {
        h.logger.Error("callback failed",
            "url", verified.CallbackURL,
            "error", err,
        )
        return h.messages.Error
    }

    // 7. Success
    h.logger.Info("verification successful",
        "phone", senderNormalized,
        "app", verified.AppName,
        "challenge_id", verified.ChallengeID,
    )
    return h.messages.Success
}
```

**`postCallback` method:**

```go
func (h *Handler) postCallback(ctx context.Context, callbackURL, jwt string) error {
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, nil)
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+jwt)
    req.Header.Set("Content-Type", "application/json")

    resp, err := h.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("execute callback: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
        return fmt.Errorf("callback returned %d: %s", resp.StatusCode, string(body))
    }
    return nil
}
```

---

### Phase 4: JWTGenerator Enhancement

**File:** `internal/auth/jwt.go` (modify existing)

Add a method to generate callback JWTs with a dynamic audience (the app name):

```go
// TokenWithAudience generates a JWT with the specified audience.
// Used for verification callbacks where the audience is the app_name from the verification token.
func (g *JWTGenerator) TokenWithAudience(userID, audience string) (string, error) {
    now := time.Now()
    claims := Claims{
        UserID:  userID,
        Channel: "whatsapp",
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    g.issuer,
            Audience:  jwt.ClaimStrings{audience},
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(g.ttl)),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(g.key)
}
```

The existing `Token(userID)` method continues to work for ADK agent requests (with the static configured audience).

---

### Phase 5: Message Router Integration

**File:** `internal/whatsapp/client.go` (modify existing)

In the WhatsApp message handler, add verification routing **before** the ADK forwarding:

```go
func (c *Client) handleMessage(evt *events.Message) {
    senderPhone := evt.Info.Sender.User  // e.g. "910987654321"
    body := evt.Message.GetConversation()

    // --- NEW: Check for verification token ---
    if c.verifyHandler != nil && auth.IsVerificationToken(body) != nil {
        response := c.verifyHandler.Handle(context.Background(), senderPhone, body)
        if response != "" {
            c.sendMessage(evt.Info.Sender, response)
            return  // do NOT forward to ADK
        }
        // If response is empty, IsVerificationToken returned nil on second check
        // (race condition safety) — fall through to ADK
    }

    // --- Existing: Forward to ADK agent ---
    c.forwardToAgent(senderPhone, body)
}
```

**Inject the handler via constructor:**

```go
type Client struct {
    // ... existing fields
    verifyHandler *verification.Handler  // nil if verification disabled
}

func NewClient(cfg Config, agentClient *agent.Client, verifyHandler *verification.Handler) *Client
```

---

### Phase 6: Startup Wiring

**File:** `cmd/gateway/main.go` (modify)

```go
func main() {
    cfg := config.Load()
    logger := slog.New(...)

    // Existing: JWT generator for ADK/callback signing
    jwtGen, err := auth.NewJWTGenerator(
        cfg.Auth.JWT.PrivateKeyPath,
        cfg.Auth.JWT.Issuer,
        cfg.Auth.JWT.Audience,
        cfg.Auth.JWT.TTL,
    )

    // NEW: Verification handler
    var verifyHandler *verification.Handler
    if cfg.Verification.Enabled {
        // Load app public keys
        keyRegistry, err := auth.NewKeyRegistry(cfg.Verification.Apps)
        if err != nil {
            logger.Error("failed to load verification app keys", "error", err)
            os.Exit(1)
        }
        logger.Info("verification enabled", "apps", len(cfg.Verification.Apps))

        timeout, _ := time.ParseDuration(cfg.Verification.CallbackTimeout)
        if timeout == 0 {
            timeout = 10 * time.Second
        }

        verifyHandler = verification.NewHandler(
            keyRegistry,
            jwtGen,
            cfg.Verification,
            &http.Client{Timeout: timeout},
            logger,
        )
    }

    // Existing: ADK agent client
    agentClient := agent.NewClient(cfg.ADK, jwtGen)

    // Pass verifyHandler to WhatsApp client (nil if disabled)
    waClient := whatsapp.NewClient(cfg.WhatsApp, agentClient, verifyHandler)

    // ... rest of startup
}
```

---

## 7. Validation Rules

### 7.1 Token Validation Checklist

| # | Check | Failure response |
|---|-------|-----------------|
| 1 | Message looks like JWT (`eyJ` prefix, 3 dot-separated parts) | Forward to ADK (not a verification token) |
| 2 | JWT contains `mobile`, `app_name`, `callback_url` claims | Forward to ADK (not a verification token) |
| 3 | `app_name` exists in key registry | Error message |
| 4 | RS256 signature valid against app's public key | Expired message |
| 5 | `exp` is in the future | Expired message |
| 6 | Sender phone matches `mobile` claim | Phone mismatch message |
| 7 | `callback_url` is HTTPS (in production) | Error message |
| 8 | Callback POST returns 2xx | Success message |
| 9 | Callback POST returns 4xx/5xx | Error message |

### 7.2 Phone Normalization

Both sender phone and JWT `mobile` claim should be compared after stripping:
- Leading `+`
- Leading `0` or `00` (country-code prefixed)
- Spaces, dashes, parentheses

```go
func normalizePhone(phone string) string {
    // Remove all non-digit characters
    digits := strings.Map(func(r rune) rune {
        if r >= '0' && r <= '9' {
            return r
        }
        return -1
    }, phone)
    return digits
}
```

### 7.3 Callback URL Validation

```go
func validateCallbackURL(raw string, requireHTTPS bool) error {
    u, err := url.Parse(raw)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }
    if u.Scheme == "" || u.Host == "" {
        return fmt.Errorf("URL must have scheme and host")
    }
    if requireHTTPS && u.Scheme != "https" {
        return fmt.Errorf("callback URL must use HTTPS")
    }
    return nil
}
```

---

## 8. Testing Plan

### 8.1 Unit Tests

| Test | File | Description |
|------|------|-------------|
| `TestIsVerificationToken` | `auth/verify_token_test.go` | Valid verification JWT → returns claims |
| `TestIsVerificationToken_NormalMessage` | `auth/verify_token_test.go` | "Hello world" → returns nil |
| `TestIsVerificationToken_ADKToken` | `auth/verify_token_test.go` | JWT without `callback_url` → returns nil |
| `TestVerifyVerificationToken` | `auth/verify_token_test.go` | Valid signature → returns claims |
| `TestVerifyVerificationToken_BadSignature` | `auth/verify_token_test.go` | Wrong key → error |
| `TestVerifyVerificationToken_Expired` | `auth/verify_token_test.go` | Expired token → error |
| `TestKeyRegistry_LoadKeys` | `auth/key_registry_test.go` | Load valid PEM files |
| `TestKeyRegistry_MissingKey` | `auth/key_registry_test.go` | Missing file → error |
| `TestKeyRegistry_UnknownApp` | `auth/key_registry_test.go` | Unknown app name → error |
| `TestHandler_SuccessFlow` | `verification/handler_test.go` | Full flow with mock HTTP server → success message |
| `TestHandler_PhoneMismatch` | `verification/handler_test.go` | Sender ≠ claim → mismatch message |
| `TestHandler_CallbackFails` | `verification/handler_test.go` | Callback returns 500 → error message |
| `TestHandler_ExpiredToken` | `verification/handler_test.go` | Expired JWT → expired message |
| `TestTokenWithAudience` | `auth/jwt_test.go` | Dynamic audience in callback JWT |

### 8.2 Integration Test

Create `test/integration/verification_test.go`:

1. Generate RSA keypair in test
2. Start mock HTTP server as "app backend callback"
3. Sign a verification JWT with the test private key
4. Call `handler.Handle(ctx, senderPhone, verificationJWT)`
5. Assert:
   - Mock server received the callback POST
   - Callback had valid `Authorization: Bearer <jwt>` header
   - Callback JWT contains correct `user_id`, `channel`, `iss`, `aud`
   - Handler returned success message

### 8.3 Manual E2E Test

1. Start the Kln API with `DEBUG_AUTH=false` and proper RSA keys
2. Start WhatsADK gateway with the Kln API's public key configured
3. Call `POST /api/v1/auth/whatsapp/init` with a test phone number
4. Copy the JWT from the `wa_link` response
5. Send the JWT from the matching WhatsApp number to the gateway
6. Verify:
   - Gateway sends success message back
   - `GET /api/v1/auth/whatsapp/status` returns `verified` with tokens

---

## 9. Security Considerations

### 9.1 Mandatory

| Concern | Mitigation |
|---------|------------|
| **Token replay** | JWT `exp` enforced (default 5 min TTL); backend marks challenge as consumed |
| **Phone spoofing** | WhatsApp provides sender identity via E2E; `mobile` claim must match sender |
| **Callback URL injection** | Validate URL format; optionally restrict to allowlisted domains per app |
| **Key compromise** | Each app has its own public key; compromising one doesn't affect others |
| **Man-in-the-middle** | Callback URL must be HTTPS in production |
| **Denial of service** | Rate-limit verification attempts per sender (e.g. 5/minute) |

### 9.2 Recommended

- **Callback URL allowlist:** Optionally configure allowed callback URL patterns per app:
  ```yaml
  apps:
    orez-laundry-app:
      public_key_path: "secrets/apps/orez-laundry-app/public.pem"
      allowed_callback_hosts:
        - "api.orezlaundry.com"
        - "staging-api.orezlaundry.com"
  ```
  Reject callbacks to any other host.

- **Rate limiting:** Track verification attempts per sender phone number. Reject after threshold:
  ```go
  type RateLimiter struct {
      attempts map[string][]time.Time  // phone → timestamps
      max      int                      // e.g. 5
      window   time.Duration            // e.g. 1 minute
  }
  ```

- **Structured logging:** Every verification attempt (success or failure) should be logged with:
  - Sender phone (last 4 digits only)
  - App name
  - Challenge ID
  - Result (success/expired/mismatch/error)
  - Duration

---

## 10. New File Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/auth/verify_token.go` | **Create** | `IsVerificationToken`, `VerifyVerificationToken` |
| `internal/auth/verify_token_test.go` | **Create** | Token detection + verification tests |
| `internal/auth/key_registry.go` | **Create** | Multi-app public key registry |
| `internal/auth/key_registry_test.go` | **Create** | Key loading tests |
| `internal/auth/jwt.go` | **Modify** | Add `TokenWithAudience` method |
| `internal/auth/jwt_test.go` | **Modify** | Add dynamic audience test |
| `internal/verification/handler.go` | **Create** | Verification message handler + callback |
| `internal/verification/handler_test.go` | **Create** | Handler unit tests |
| `internal/config/config.go` | **Modify** | Add `VerificationConfig` struct |
| `internal/whatsapp/client.go` | **Modify** | Route verification messages to handler |
| `cmd/gateway/main.go` | **Modify** | Wire verification handler at startup |
| `config/config.yaml` | **Modify** | Add `verification:` section |

---

## 11. Deployment Checklist

1. **Generate keys** for each app that will use verification:
   ```sh
   # On the app backend side
   openssl genrsa -out jwt_private.pem 2048
   openssl rsa -in jwt_private.pem -pubout -out jwt_public.pem
   ```

2. **Copy app public keys** to the gateway:
   ```sh
   mkdir -p secrets/apps/orez-laundry-app/
   cp /from/app/jwt_public.pem secrets/apps/orez-laundry-app/public.pem
   chmod 644 secrets/apps/orez-laundry-app/public.pem
   ```

3. **Copy gateway public key** to each app backend:
   ```sh
   openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
   cp secrets/jwt_public.pem /to/app/secrets/whatsadk_public.pem
   ```

4. **Update gateway config** (`config.yaml`):
   ```yaml
   verification:
     enabled: true
     callback_timeout: "10s"
     apps:
       orez-laundry-app:
         public_key_path: "secrets/apps/orez-laundry-app/public.pem"
   ```

5. **Update app backend `.env`:**
   ```
   WHATSAPP_VERIFY_PRIVATE_KEY_PATH=secrets/jwt_private.pem
   WHATSADK_PUBLIC_KEY_PATH=secrets/whatsadk_public.pem
   WHATSADK_EXPECTED_ISSUER=whatsadk-gateway
   WHATSADK_EXPECTED_AUDIENCE=orez-laundry-app
   WHATSAPP_BACKEND_NUMBER=910000000000
   ```

6. **Verify key pair match:**
   ```sh
   # These should produce identical output
   openssl rsa -in secrets/jwt_private.pem -pubout -outform DER | sha256sum
   openssl rsa -pubin -in secrets/apps/orez-laundry-app/public.pem -outform DER | sha256sum
   ```

7. **Test end-to-end** with a real WhatsApp number before going live.

---

## 12. Estimated Effort

| Phase | Description | Estimate |
|-------|-------------|----------|
| 1 | Config + key registry | 2–3 hours |
| 2 | Token parser + detection | 2–3 hours |
| 3 | Verification handler + callback | 3–4 hours |
| 4 | JWT generator enhancement | 30 minutes |
| 5 | Message router integration | 1–2 hours |
| 6 | Startup wiring | 1 hour |
| — | Unit + integration tests | 3–4 hours |
| — | Manual E2E testing | 1–2 hours |
| **Total** | | **~2 days** |
