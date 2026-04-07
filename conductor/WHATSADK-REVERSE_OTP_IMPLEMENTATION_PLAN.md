# WhatsADK — Reverse OTP Verification Implementation Plan

> Implementation plan for adding Reverse OTP (Mobile-Originated Verification) support to the [WhatsADK gateway](https://github.com/innomon/whatsadk).

---

## 1. Overview

The WhatsADK gateway currently bridges WhatsApp messages to Google ADK agents. This plan adds a **verification message handler** that intercepts specially formatted JWT messages from users, validates them, calls back the originating application's backend to obtain an OTP, and relays that OTP to the user via WhatsApp reply.

This is the gateway-side counterpart to the backend flow documented in the [orez-hyper-local implementation plan](https://github.com/innomon/orez-hyper-local/blob/main/IMPLEMENTATION_PLAN.md).

### How it works (summary)

1. User opens the PWA → no valid JWT → "Login/Signup with WhatsApp" modal
2. User enters WhatsApp number → backend checks user status
3. If active or new user → backend generates a JWT (signed with app's RSA private key) containing phone, TTL, and app_name → presents a WhatsApp deep link with the JWT as message body
4. User clicks the link → WhatsApp opens → user sends the JWT to the gateway's number
5. Gateway verifies the JWT, checks sender phone match and blacklist → calls the backend callback with a gateway-signed JWT
6. Backend responds with an OTP → gateway relays the OTP to the user via WhatsApp reply
7. User enters the OTP in the PWA login screen → gains access
8. If the number was not found in the database (new user), a profile capture screen is presented after OTP entry

**This is a two-factor flow:** WhatsApp ownership (sending the message proves the user controls that WhatsApp number) + OTP entry (proves the user has access to both WhatsApp and the browser session).

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
does JWT contain `mobile` + `app_name` + `challenge_id` claims?  ──no──► forward to ADK agent
    │ yes
    ▼
verification flow
```

The JWT is parsed without signature verification first (header-only or claims peek) to check for verification-specific claims. Full RS256 signature verification happens only after identifying it as a verification token.

> **Security note — no `callback_url` in the JWT:** The callback destination is **not** embedded in the app-signed JWT. Instead, the gateway derives it from static per-app config (`apps[app_name].callback_base_url`). This prevents SSRF attacks where an attacker who obtains a legitimately signed token could cause the gateway to POST credentials to an attacker-controlled URL.

### 3.2 Verification Flow

```
┌──────────┐        ┌───────────────┐        ┌──────────────┐
│ WhatsApp │        │  WhatsADK GW  │        │ App Backend  │
│ (User)   │        │               │        │ (orez API)   │
└────┬─────┘        └──────┬────────┘        └──────┬───────┘
     │                      │                        │
     │  1. User opens PWA,  │                        │
     │     enters phone     │                        │
     │     number           │                        │
     │                      │                        │
     │                      │                        │ 2. Backend checks
     │                      │                        │    user status
     │                      │                        │    (pending/suspended
     │                      │                        │     → stop;
     │                      │                        │     active/new → ok)
     │                      │                        │
     │                      │                        │ 3. Backend generates
     │                      │                        │    JWT with phone,
     │                      │                        │    TTL, app_name
     │                      │                        │
     │                      │                        │ 4. WhatsApp deep link
     │  ◄────────────────────────────────────────────┤    presented to user
     │                      │                        │
     │ 5. User clicks link, │                        │
     │    sends JWT message  │                        │
     ├─────────────────────►│                        │
     │                      │ 6. Detect JWT          │
     │                      │ 7. Parse claims        │
     │                      │ 8. Load app public key │
     │                      │ 9. Verify RS256 sig    │
     │                      │ 10. Check exp          │
     │                      │ 11. Match sender phone │
     │                      │     vs claim.mobile    │
     │                      │ 12. Check blacklist    │
     │                      │ 13. Resolve callback   │
     │                      │     URL from app config│
     │                      │                        │
     │                      │ 14. Sign callback JWT  │
     │                      │     (GW private key,   │
     │                      │      incl challenge_id)│
     │                      │                        │
     │                      │ POST callback_base_url │
     │                      │  + /callback?          │
     │                      │  challenge_id=...      │
     │                      ├───────────────────────►│
     │                      │                        │ 15. Verify GW JWT
     │                      │                        │ 16. Generate OTP
     │                      │                        │ 17. Store OTP with
     │                      │                        │     challenge
     │                      │◄───────────────────────┤
     │                      │ 200 OK + {"otp":"..."}│
     │                      │                        │
     │ 18. Gateway sends OTP│                        │
     │     via WhatsApp     │                        │
     │     reply            │                        │
     │◄─────────────────────┤                        │
     │                      │                        │
     │  "Your OTP is: 1234" │                        │
     │                      │                        │
     │ 19. User enters OTP  │                        │
     │     in PWA login     │                        │
     │     screen           ├ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ │
     │                      │                        │ 20. Backend verifies
     │                      │                        │     OTP, issues
     │                      │                        │     session JWT
     │                      │                        │
     │                      │                        │ 21. If new user →
     │                      │                        │     profile capture
     │                      │                        │     screen
```

### 3.3 DevOps Override

If the sender phone does not match the `mobile` claim in the JWT, the gateway checks whether the sender's number is flagged as a **DevOps** number in the configuration. DevOps numbers are allowed to proceed regardless of phone mismatch — this supports testing and operational scenarios where a DevOps operator needs to verify on behalf of another number.

If the sender is neither matching nor DevOps, the gateway sends an error message back via WhatsApp.

---

## 4. Specification

### 4.1 Incoming Verification JWT (from app backend → user → gateway)

**Algorithm:** RS256
**Signed by:** App backend's RSA private key
**Verified by:** Gateway using app's public key

**Claims:**

| Claim | Type | Required | Description |
|-------|------|----------|-------------|
| `mobile` | string | ✅ | User's phone number in E.164 digits (no `+`), e.g. `"910987654321"` |
| `app_name` | string | ✅ | Application identifier, lowercase-with-dashes (e.g. `"orez-hyper-local"`) |
| `challenge_id` | string | ✅ | Unique challenge identifier (UUID) |
| `iat` | number | ✅ | Issued at (Unix timestamp) |
| `exp` | number | ✅ | Expiry (Unix timestamp) |

> **Removed:** `callback_url` is no longer included in this JWT. The gateway resolves the callback destination from static per-app config (`callback_base_url`). See §5.1.

**Example decoded payload:**
```json
{
  "mobile": "910987654321",
  "app_name": "orez-hyper-local",
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
| `user_id` | string | Sender's WhatsApp phone number in E.164 digits (no `+`), e.g. `"910987654321"` |
| `channel` | string | `"whatsapp"` |
| `challenge_id` | string | Challenge ID from the incoming verification JWT — binds this callback to a specific challenge |
| `iss` | string | Gateway issuer (e.g. `"whatsadk-gateway"`) |
| `aud` | string | App name from the verification token (e.g. `"orez-hyper-local"`) |
| `iat` | number | Issued at (Unix timestamp) |
| `exp` | number | Expiry (Unix timestamp, short TTL — 2 minutes) |

**Example decoded payload:**
```json
{
  "user_id": "910987654321",
  "channel": "whatsapp",
  "challenge_id": "abc-123",
  "iss": "whatsadk-gateway",
  "aud": "orez-hyper-local",
  "iat": 1739000010,
  "exp": 1739000130
}
```

### 4.3 Callback HTTP Request

The callback URL is constructed by the gateway from static config:
```
<callback_base_url>/callback?challenge_id=<challenge_id>
```

```
POST <constructed_callback_url>
Authorization: Bearer <gateway_jwt>
Content-Type: application/json

(empty body)
```

The gateway JWT in the `Authorization` header contains `challenge_id` cryptographically bound to the token. The backend **must** verify that the `challenge_id` in the JWT matches the `challenge_id` in the URL query parameter.

**Expected responses:**

| Status | Meaning | Response Body | Gateway action |
|--------|---------|---------------|----------------|
| `200` | Verification accepted, OTP generated | `{"otp": "123456"}` | Send OTP to user via WhatsApp |
| `400` | Challenge invalid/expired | — | Send failure message to user |
| `401` | JWT rejected | — | Send failure message to user |
| `5xx` | Server error | — | Send retry message or failure to user |

### 4.4 WhatsApp Response Messages

**OTP delivery (success):**
```
🔐 Your verification code is: <OTP>

Enter this code in the app to complete login. This code expires in 5 minutes.
```

**Failure (invalid/expired token):**
```
❌ Verification failed. The link may have expired. Please request a new one from the app.
```

**Failure (phone mismatch — non-DevOps sender):**
```
❌ Verification failed. Please make sure you're sending from the same number you registered with.
```

**Failure (blacklisted number):**
```
🚫 This number has been blocked. Please contact support for assistance.
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
    orez-hyper-local:
      public_key_path: "secrets/apps/orez-hyper-local/public.pem"
      callback_base_url: "https://api.qzip.in/api/v1/auth/whatsapp"
    another-app:
      public_key_path: "secrets/apps/another-app/public.pem"
      callback_base_url: "https://api.another-app.com/api/v1/auth/whatsapp"
  store_path: "gateway.db"       # SQLite database for blacklisted numbers
  devops_numbers:
    - "919999999999"    # DevOps numbers allowed to bypass phone mismatch
  messages:
    otp_delivery: "🔐 Your verification code is: %s\n\nEnter this code in the app to complete login. This code expires in 5 minutes."
    expired: "❌ Verification failed. The link may have expired. Please request a new one from the app."
    phone_mismatch: "❌ Verification failed. Please make sure you're sending from the same number you registered with."
    blacklisted: "🚫 This number has been blocked from verification."
    error: "⚠️ Something went wrong. Please try again in a moment."
```

**Design rationale — database-backed blacklist:** Blacklisted numbers are stored in a SQLite database (`gateway.db` by default) rather than static YAML config. The `blacklisted_numbers` table (with `phone`, `reason`, `created_at` columns) is auto-created on startup. This allows adding/removing numbers at runtime without restarting the gateway. Numbers use E.164 digits format (e.g. `"910987654321"`).

**Design rationale — multi-app support:** The gateway may serve multiple applications simultaneously. Each app is identified by `app_name` in the verification JWT, and the gateway looks up the corresponding public key. This avoids deploying separate gateway instances per app.

### 5.2 Environment Variable Overrides

| Variable | Description |
|----------|-------------|
| `VERIFICATION_ENABLED` | `true`/`false` — master switch |
| `VERIFICATION_CALLBACK_TIMEOUT` | HTTP timeout for callback requests (default `10s`) |
| `VERIFICATION_APP_<NAME>_PUBLIC_KEY_PATH` | Public key path per app (e.g. `VERIFICATION_APP_OREZ_HYPER_LOCAL_PUBLIC_KEY_PATH`) |
| `VERIFICATION_APP_<NAME>_CALLBACK_BASE_URL` | Callback base URL per app (e.g. `VERIFICATION_APP_OREZ_HYPER_LOCAL_CALLBACK_BASE_URL`) |

### 5.3 Key File Organization

```
secrets/
├── jwt_private.pem              # Gateway's own private key (existing, for signing callback JWTs)
└── apps/
    ├── orez-hyper-local/
    │   └── public.pem           # orez-hyper-local's RS256 public key
    └── another-app/
        └── public.pem           # another-app's RS256 public key
```

---

## 6. Implementation Plan

### Phase 1: Verification Config + Key Registry + Blacklist

**File:** `internal/config/config.go`

Add verification configuration structs:

```go
type VerificationConfig struct {
    Enabled         bool                       `yaml:"enabled"`
    CallbackTimeout string                     `yaml:"callback_timeout"` // e.g. "10s"
    Apps            map[string]AppVerifyConfig  `yaml:"apps"`
    Blacklist       []string                   `yaml:"blacklist"`
    DevOpsNumbers   []string                   `yaml:"devops_numbers"`
    Messages        VerificationMessages        `yaml:"messages"`
}

type AppVerifyConfig struct {
    PublicKeyPath   string `yaml:"public_key_path"`
    CallbackBaseURL string `yaml:"callback_base_url"` // e.g. "https://api.qzip.in/api/v1/auth/whatsapp"
}

type VerificationMessages struct {
    OTPDelivery   string `yaml:"otp_delivery"`
    Expired       string `yaml:"expired"`
    PhoneMismatch string `yaml:"phone_mismatch"`
    Blacklisted   string `yaml:"blacklisted"`
    Error         string `yaml:"error"`
}
```

Add `Verification VerificationConfig` to the root `Config` struct.

**File:** `internal/auth/key_registry.go` (new)

```go
type KeyRegistry struct {
    appKeys          map[string]*rsa.PublicKey  // app_name → public key
    callbackBaseURLs map[string]string          // app_name → callback base URL
    blacklist        map[string]bool            // phone → blocked
    devOpsNumbers    map[string]bool            // phone → is devops
}

func NewKeyRegistry(apps map[string]AppVerifyConfig, blacklist []string, devOpsNumbers []string) (*KeyRegistry, error)
func (r *KeyRegistry) GetAppPublicKey(appName string) (*rsa.PublicKey, error)
func (r *KeyRegistry) GetCallbackBaseURL(appName string) (string, error)
func (r *KeyRegistry) IsBlacklisted(phone string) bool
func (r *KeyRegistry) IsDevOps(phone string) bool
```

Load all app public keys at startup. Return a clear error if any key file is missing or unparseable. Blacklist and DevOps numbers are stored as sets for O(1) lookup.

---

### Phase 2: Verification Token Parser

**File:** `internal/auth/verify_token.go` (new)

```go
// VerificationClaims represents the JWT claims from an app backend
type VerificationClaims struct {
    Mobile      string `json:"mobile"`
    AppName     string `json:"app_name"`
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
2. Check that `mobile`, `app_name`, and `challenge_id` are all non-empty
3. If yes → return claims (for routing); if no → return nil

**`VerifyVerificationToken` logic:**
1. `jwt.ParseWithClaims()` with RS256 method enforcement
2. Validate `exp` is in the future
3. Validate `mobile`, `app_name`, `challenge_id` are non-empty
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

func NewHandler(keys *auth.KeyRegistry, jwtGen *auth.JWTGenerator, cfg VerificationConfig, httpClient *http.Client, logger *slog.Logger) *Handler

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

    // 2. Check blacklist
    senderNormalized := normalizePhone(senderPhone)
    if h.keys.IsBlacklisted(senderNormalized) {
        h.logger.Warn("blacklisted number attempted verification",
            "sender", senderNormalized,
        )
        return h.messages.Blacklisted
    }

    // 3. Look up app public key
    appKey, err := h.keys.GetAppPublicKey(claims.AppName)
    if err != nil {
        h.logger.Warn("unknown app", "app_name", claims.AppName)
        return h.messages.Error
    }

    // 4. Full signature verification
    verified, err := auth.VerifyVerificationToken(messageBody, appKey)
    if err != nil {
        h.logger.Warn("verification token invalid", "error", err, "app", claims.AppName)
        return h.messages.Expired
    }

    // 5. Phone number match (E.164 digits, exact comparison)
    //    DevOps numbers bypass the phone mismatch check
    claimNormalized := normalizePhone(verified.Mobile)
    if senderNormalized != claimNormalized {
        if !h.keys.IsDevOps(senderNormalized) {
            h.logger.Warn("phone mismatch",
                "sender", senderNormalized,
                "claim_mobile", claimNormalized,
            )
            return h.messages.PhoneMismatch
        }
        h.logger.Info("devops override: phone mismatch allowed",
            "sender", senderNormalized,
            "claim_mobile", claimNormalized,
        )
    }

    // 6. Resolve callback URL from static config (NOT from JWT)
    callbackBaseURL, err := h.keys.GetCallbackBaseURL(verified.AppName)
    if err != nil {
        h.logger.Error("no callback URL configured", "app", verified.AppName)
        return h.messages.Error
    }
    callbackURL := callbackBaseURL + "/callback?challenge_id=" + url.QueryEscape(verified.ChallengeID)

    // 7. Sign callback JWT (includes challenge_id)
    callbackJWT, err := h.jwtGen.TokenWithAudienceAndChallenge(senderNormalized, verified.AppName, verified.ChallengeID)
    if err != nil {
        h.logger.Error("failed to sign callback JWT", "error", err)
        return h.messages.Error
    }

    // 8. POST to callback URL and receive OTP
    otp, err := h.postCallback(ctx, callbackURL, callbackJWT)
    if err != nil {
        h.logger.Error("callback failed",
            "url", callbackURL,
            "error", err,
        )
        return h.messages.Error
    }

    // 9. Send OTP to user via WhatsApp reply
    h.logger.Info("verification successful, OTP relayed",
        "phone", senderNormalized,
        "app", verified.AppName,
        "challenge_id", verified.ChallengeID,
    )
    return fmt.Sprintf(h.messages.OTPDelivery, otp)
}
```

**`postCallback` method:**

```go
// callbackResponse represents the JSON response from the app backend callback.
type callbackResponse struct {
    OTP string `json:"otp"`
}

func (h *Handler) postCallback(ctx context.Context, callbackURL, jwt string) (string, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, nil)
    if err != nil {
        return "", fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+jwt)
    req.Header.Set("Content-Type", "application/json")

    resp, err := h.httpClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("execute callback: %w", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

    if resp.StatusCode >= 400 {
        return "", fmt.Errorf("callback returned %d: %s", resp.StatusCode, string(body))
    }

    var cbResp callbackResponse
    if err := json.Unmarshal(body, &cbResp); err != nil {
        return "", fmt.Errorf("parse callback response: %w", err)
    }
    if cbResp.OTP == "" {
        return "", fmt.Errorf("callback response missing otp field")
    }

    return cbResp.OTP, nil
}
```

---

### Phase 4: JWTGenerator Enhancement

**File:** `internal/auth/jwt.go` (modify existing)

Add a method to generate callback JWTs with a dynamic audience (the app name):

```go
// TokenWithAudienceAndChallenge generates a JWT with the specified audience and challenge_id.
// Used for verification callbacks — binds the gateway assertion to a specific challenge.
func (g *JWTGenerator) TokenWithAudienceAndChallenge(userID, audience, challengeID string) (string, error) {
    now := time.Now()
    claims := Claims{
        UserID:      userID,
        Channel:     "whatsapp",
        ChallengeID: challengeID,
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
        // Load app public keys, blacklist, and devops numbers
        keyRegistry, err := auth.NewKeyRegistry(cfg.Verification.Apps, cfg.Verification.Blacklist, cfg.Verification.DevOpsNumbers)
        if err != nil {
            logger.Error("failed to load verification app keys", "error", err)
            os.Exit(1)
        }
        logger.Info("verification enabled",
            "apps", len(cfg.Verification.Apps),
            "blacklisted", len(cfg.Verification.Blacklist),
            "devops_numbers", len(cfg.Verification.DevOpsNumbers),
        )

        timeout, _ := time.ParseDuration(cfg.Verification.CallbackTimeout)
        if timeout == 0 {
            timeout = 10 * time.Second
        }

        verifyHandler = verification.NewHandler(
            keyRegistry,
            jwtGen,
            cfg.Verification,
            &http.Client{
                Timeout: timeout,
                CheckRedirect: func(req *http.Request, via []*http.Request) error {
                    return fmt.Errorf("redirects not allowed for verification callbacks")
                },
            },
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
| 2 | JWT contains `mobile`, `app_name`, `challenge_id` claims | Forward to ADK (not a verification token) |
| 3 | Sender phone is not blacklisted | Blacklisted message |
| 4 | `app_name` exists in key registry (with public key + `callback_base_url`) | Error message |
| 5 | RS256 signature valid against app's public key | Expired message |
| 6 | `exp` is in the future | Expired message |
| 7 | Sender phone matches `mobile` claim (E.164 digits comparison), OR sender is a DevOps number | Phone mismatch message |
| 8 | Callback POST returns 2xx with `{"otp":"..."}` | Send OTP via WhatsApp reply |
| 9 | Callback POST returns 4xx/5xx | Error message |

### 7.2 Phone Normalization

Both sender phone and JWT `mobile` claim are compared as **E.164 digits** (country code + number, no `+` prefix). The app backend **must** store the `mobile` claim in the same format WhatsApp uses for sender identity.

Normalization strips only non-digit characters — no leading-zero manipulation (which could collapse distinct numbers from different countries):

```go
func normalizePhone(phone string) string {
    // Remove all non-digit characters ('+', spaces, dashes, parens)
    digits := strings.Map(func(r rune) rune {
        if r >= '0' && r <= '9' {
            return r
        }
        return -1
    }, phone)
    return digits
}
```

### 7.3 Callback URL Safety

Since callback URLs are now derived from **static config** (not from JWT claims), the primary SSRF risk is eliminated. However, the gateway HTTP client should still follow defensive practices:

```go
httpClient := &http.Client{
    Timeout: timeout,
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        return fmt.Errorf("redirects not allowed for verification callbacks")
    },
}
```

At startup, validate that all configured `callback_base_url` values use HTTPS (except in development mode).

---

## 8. Testing Plan

### 8.1 Unit Tests

| Test | File | Description |
|------|------|-------------|
| `TestIsVerificationToken` | `auth/verify_token_test.go` | Valid verification JWT → returns claims |
| `TestIsVerificationToken_NormalMessage` | `auth/verify_token_test.go` | "Hello world" → returns nil |
| `TestIsVerificationToken_ADKToken` | `auth/verify_token_test.go` | JWT without `challenge_id` → returns nil |
| `TestVerifyVerificationToken` | `auth/verify_token_test.go` | Valid signature → returns claims |
| `TestVerifyVerificationToken_BadSignature` | `auth/verify_token_test.go` | Wrong key → error |
| `TestVerifyVerificationToken_Expired` | `auth/verify_token_test.go` | Expired token → error |
| `TestKeyRegistry_LoadKeys` | `auth/key_registry_test.go` | Load valid PEM files |
| `TestKeyRegistry_MissingKey` | `auth/key_registry_test.go` | Missing file → error |
| `TestKeyRegistry_UnknownApp` | `auth/key_registry_test.go` | Unknown app name → error |
| `TestKeyRegistry_IsBlacklisted` | `auth/key_registry_test.go` | Blacklisted number → true |
| `TestKeyRegistry_IsDevOps` | `auth/key_registry_test.go` | DevOps number → true |
| `TestHandler_SuccessFlow` | `verification/handler_test.go` | Full flow with mock HTTP server → OTP relayed |
| `TestHandler_BlacklistedNumber` | `verification/handler_test.go` | Blacklisted sender → blocked message |
| `TestHandler_PhoneMismatch` | `verification/handler_test.go` | Sender ≠ claim (non-DevOps) → mismatch message |
| `TestHandler_PhoneMismatch_DevOps` | `verification/handler_test.go` | Sender ≠ claim (DevOps) → allowed, OTP relayed |
| `TestHandler_CallbackReturnsOTP` | `verification/handler_test.go` | Callback returns `{"otp":"123456"}` → OTP message sent |
| `TestHandler_CallbackMissingOTP` | `verification/handler_test.go` | Callback returns `{}` → error message |
| `TestHandler_CallbackFails` | `verification/handler_test.go` | Callback returns 500 → error message |
| `TestHandler_ExpiredToken` | `verification/handler_test.go` | Expired JWT → expired message |
| `TestTokenWithAudienceAndChallenge` | `auth/jwt_test.go` | Dynamic audience + challenge_id in callback JWT |

### 8.2 Integration Test

Create `test/integration/verification_test.go`:

1. Generate RSA keypair in test
2. Start mock HTTP server as "app backend callback" that returns `{"otp":"654321"}`
3. Sign a verification JWT with the test private key
4. Call `handler.Handle(ctx, senderPhone, verificationJWT)`
5. Assert:
   - Mock server received the callback POST
   - Callback had valid `Authorization: Bearer <jwt>` header
   - Callback JWT contains correct `user_id`, `channel`, `challenge_id`, `iss`, `aud`
   - Handler returned OTP delivery message containing "654321"
6. Test blacklisted number → no callback made, blocked message returned
7. Test DevOps override → callback made despite phone mismatch

### 8.3 Manual E2E Test

1. Start the orez-hyper-local API with proper RSA keys
2. Start WhatsADK gateway with the app's public key configured
3. Open the PWA → enter a test phone number → get the WhatsApp deep link
4. Click the link → send the JWT from the matching WhatsApp number to the gateway
5. Verify:
   - Gateway sends OTP message back via WhatsApp
   - Enter the OTP in the PWA login screen → access granted
   - For a new (unregistered) number: profile capture screen appears after OTP entry
6. Test with a blacklisted number → verify blocked message received
7. Test with a DevOps number sending on behalf of another phone → verify OTP received

---

## 9. Security Considerations

### 9.1 Mandatory

| Concern | Mitigation |
|---------|------------|
| **Token replay** | JWT `exp` enforced (default 5 min TTL); backend marks challenge as consumed; `challenge_id` bound in gateway JWT prevents cross-challenge replay |
| **Phone spoofing** | WhatsApp provides sender identity via E2E; `mobile` claim must match sender (E.164 exact comparison) |
| **SSRF / Callback URL injection** | Callback URL derived from **static per-app config**, not from JWT claims. No runtime URL injection possible |
| **Confused deputy** | Gateway JWT includes `challenge_id` — backend must verify it matches the pending challenge |
| **Key compromise** | Each app has its own public key; compromising one doesn't affect others |
| **Man-in-the-middle** | Callback URL must be HTTPS in production; redirects disallowed |
| **Denial of service** | Rate-limit verification attempts per sender (e.g. 5/minute) |
| **Number blacklisting** | Gateway checks sender against blacklist before processing; blocked numbers never trigger a callback |
| **OTP interception** | OTP delivered via WhatsApp (E2E encrypted) and entered in the PWA — two separate channels; attacker needs access to both |
| **Two-factor assurance** | Factor 1: WhatsApp message (proves phone ownership); Factor 2: OTP entry in PWA (proves browser session continuity) |

### 9.2 Recommended

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
  - Result (success/expired/mismatch/blacklisted/error)
  - Duration

- **OTP security on backend:** The backend should generate cryptographically random OTPs (6 digits), hash them before storage, and enforce single-use with expiry (5 minutes). The gateway treats the OTP as an opaque string — it only relays it.

---

## 10. New File Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/auth/verify_token.go` | **Create** | `IsVerificationToken`, `VerifyVerificationToken` |
| `internal/auth/verify_token_test.go` | **Create** | Token detection + verification tests |
| `internal/auth/key_registry.go` | **Create** | Multi-app public key registry + DevOps lookup |
| `internal/auth/key_registry_test.go` | **Create** | Key loading, DevOps tests |
| `internal/store/store.go` | **Create** | SQLite-backed blacklist store (`blacklisted_numbers` table) |
| `internal/store/store_test.go` | **Create** | Blacklist CRUD tests |
| `internal/auth/jwt.go` | **Modify** | Add `TokenWithAudienceAndChallenge` method |
| `internal/auth/jwt_test.go` | **Modify** | Add dynamic audience test |
| `internal/verification/handler.go` | **Create** | Verification message handler + OTP relay via callback |
| `internal/verification/handler_test.go` | **Create** | Handler unit tests (incl. OTP, blacklist, DevOps) |
| `internal/config/config.go` | **Modify** | Add `VerificationConfig` struct with blacklist/DevOps fields |
| `internal/whatsapp/client.go` | **Modify** | Route verification messages to handler |
| `cmd/gateway/main.go` | **Modify** | Wire verification handler at startup |
| `config/config.yaml` | **Modify** | Add `verification:` section with blacklist + DevOps config |

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
   mkdir -p secrets/apps/orez-hyper-local/
   cp /from/app/jwt_public.pem secrets/apps/orez-hyper-local/public.pem
   chmod 644 secrets/apps/orez-hyper-local/public.pem
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
     store_path: "gateway.db"
     apps:
       orez-hyper-local:
         public_key_path: "secrets/apps/orez-hyper-local/public.pem"
         callback_base_url: "https://api.qzip.in/api/v1/auth/whatsapp"
     devops_numbers:
       - "919999999999"
   ```

5. **Update app backend `.env`:**
   ```
   WHATSAPP_VERIFY_PRIVATE_KEY_PATH=secrets/jwt_private.pem
   WHATSADK_PUBLIC_KEY_PATH=secrets/whatsadk_public.pem
   WHATSADK_EXPECTED_ISSUER=whatsadk-gateway
   WHATSADK_EXPECTED_AUDIENCE=orez-hyper-local
   WHATSAPP_BACKEND_NUMBER=910000000000
   ```

6. **Verify key pair match:**
   ```sh
   # These should produce identical output
   openssl rsa -in secrets/jwt_private.pem -pubout -outform DER | sha256sum
   openssl rsa -pubin -in secrets/apps/orez-hyper-local/public.pem -outform DER | sha256sum
   ```

7. **Test end-to-end** with a real WhatsApp number before going live.

---

## 12. Estimated Effort

| Phase | Description | Estimate |
|-------|-------------|----------|
| 1 | Config + key registry + blacklist/DevOps | 3–4 hours |
| 2 | Token parser + detection | 2–3 hours |
| 3 | Verification handler + OTP relay callback | 3–4 hours |
| 4 | JWT generator enhancement | 30 minutes |
| 5 | Message router integration | 1–2 hours |
| 6 | Startup wiring | 1 hour |
| — | Unit + integration tests | 4–5 hours |
| — | Manual E2E testing | 1–2 hours |
| **Total** | | **~2.5 days** |
