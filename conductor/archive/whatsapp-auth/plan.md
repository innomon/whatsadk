# WhatsApp OAuth Implementation Plan (Ed25519 / EdDSA JWT)

## Phase 1: Core Auth Module

1. **No new dependencies required.**
   - `golang-jwt/jwt/v5` is already in `go.mod`.
   - `filippo.io/edwards25519` is already an indirect dependency.
   - `crypto/ed25519` is in the Go standard library.

2. **Implement `internal/auth/eddsa.go`:**
   - `LoadEdDSAKey(path string) (ed25519.PrivateKey, error)` — loads an Ed25519 private key from a PEM file (PKCS#8) or raw 64-byte seed file.
   - `EdDSAPublicKeyBase64(key ed25519.PrivateKey) string` — returns the base64url-encoded public key (for sharing with ADK server).

3. **Implement `internal/auth/oauth_token.go`:**
   - `OAuthTokenGenerator` struct holding the Ed25519 private key, issuer, audience, and TTL.
   - `NewOAuthTokenGenerator(keyPath, issuer, audience string, ttl time.Duration) (*OAuthTokenGenerator, error)`
   - `Token(phone, nonce, userPubKey string) (string, error)` — creates and signs a JWT with:
     - `sub`: phone number
     - `iss`: gateway identifier
     - `aud`: ADK server identifier
     - `iat`, `exp`: timestamps
     - `nonce`: from the auth request
     - `pubkey`: the SPA's Ed25519 public key

## Phase 2: Configuration Update

1. **Update `internal/config/config.go`:**
   - Add `OAuth` field to `AuthConfig`:
     ```go
     type AuthConfig struct {
         JWT   JWTConfig   `yaml:"jwt"`
         OAuth OAuthConfig `yaml:"oauth"`
     }

     type OAuthConfig struct {
         Enabled    bool   `yaml:"enabled"`
         KeyPath    string `yaml:"key_path"`
         SPAURL     string `yaml:"spa_url"`
         Issuer     string `yaml:"issuer"`
         Audience   string `yaml:"audience"`
         TTL        string `yaml:"ttl"`
         RateLimit  int    `yaml:"rate_limit"`
     }
     ```
   - Add env overrides: `OAUTH_ENABLED`, `OAUTH_KEY_PATH`, `OAUTH_SPA_URL`.
   - Add defaults: `ttl: "24h"`, `rate_limit: 5`, `issuer: "whatsadk-gateway"`.

2. **Update `config/config.yaml`** with commented-out defaults:
   ```yaml
   auth:
     oauth:
       enabled: false
       # key_path: "secrets/oauth_ed25519.pem"
       # spa_url: "https://chat.myadk.app"
       # issuer: "whatsadk-gateway"
       # audience: "adk-cloud-proxy"
       # ttl: "24h"
       # rate_limit: 5  # max AUTH requests per phone per hour
   ```

## Phase 3: WhatsApp Command Handling

1. **Create `internal/auth/oauth_handler.go`:**
   - `OAuthHandler` struct with `tokenGen *OAuthTokenGenerator`, `spaURL string`, `rateLimiter` (in-memory map + mutex).
   - `NewOAuthHandler(tokenGen *OAuthTokenGenerator, spaURL string, rateLimit int) *OAuthHandler`
   - `Handle(senderPhone, messageBody string) (string, error)`:
     - Parses `messageBody` with regex `^AUTH\s+([A-Za-z0-9_-]{43}=?)\s+([A-Za-z0-9_-]{16,})$`.
     - Validates `userPubKey` is a valid 32-byte base64url-encoded key.
     - Checks rate limit for `senderPhone`.
     - Generates JWT via `tokenGen.Token(senderPhone, nonce, userPubKey)`.
     - Returns WhatsApp reply message containing the deep link:
       `Click here to complete login:\n<SPA_URL>/auth#token=<JWT>&nonce=<nonce>`
   - `IsAuthCommand(text string) bool` — quick prefix check (`strings.HasPrefix(upper, "AUTH ")`) for use in the message routing pipeline.

2. **Update `internal/whatsapp/client.go`:**
   - Add `oauthHandler *auth.OAuthHandler` field to `Client` struct.
   - Update `New()` to accept `oauthHandler` parameter.
   - In `handleMessage`, add OAuth check **after** verification but **before** access control:
     ```
     verification → oauth → access control → ADK agent
     ```
   - If `oauthHandler != nil && oauthHandler.IsAuthCommand(text)`:
     - Call `oauthHandler.Handle(userID, text)`.
     - Send the result as a WhatsApp reply.
     - Return (do not forward to ADK).

## Phase 4: Integration & Startup

1. **Update `cmd/gateway/main.go`:**
   - If `cfg.Auth.OAuth.Enabled`:
     - Load Ed25519 private key via `auth.LoadEdDSAKey(cfg.Auth.OAuth.KeyPath)`.
     - Create `OAuthTokenGenerator`.
     - Create `OAuthHandler`.
     - Log: `🔑 WhatsApp OAuth enabled (EdDSA)`
   - Pass `oauthHandler` (or `nil`) to `whatsapp.New()`.

## Phase 5: Key Generation Helper

1. **Add `cmd/keygen/main.go`:**
   - CLI tool to generate an Ed25519 key pair for the gateway:
     ```bash
     go run ./cmd/keygen -out secrets/oauth_ed25519.pem
     ```
   - Outputs: PEM-encoded private key and prints the base64url public key (for configuring the ADK server).

## Phase 6: Testing & Validation

1. **Unit Tests (`internal/auth/`):**
   - `eddsa_test.go`: Test key loading from PEM and raw seed.
   - `oauth_token_test.go`: Generate a JWT → parse it back → verify claims are correct and signature is valid.
   - `oauth_handler_test.go`:
     - Valid AUTH command → returns deep link with valid JWT.
     - Invalid public key → returns error message.
     - Malformed command → returns error.
     - Rate limit exceeded → returns rate-limit message.
2. **Integration Test:**
   - Generate a key pair → create handler → simulate AUTH message → verify the returned URL contains a valid JWT with correct `sub`, `nonce`, and `pubkey` claims.
3. **Manual E2E:**
   - Generate keys with `cmd/keygen`.
   - Send `AUTH <pubkey> <nonce>` from a test WhatsApp number.
   - Verify the returned link opens the SPA and the JWT decodes correctly at `jwt.io`.

## Phase 7: Documentation

1. **Update `README.md`** with:
   - WhatsApp OAuth setup instructions (key generation, config).
   - How to provide the public key to the ADK server.
2. **Update `ARCHITECTURE.md`** with the OAuth flow diagram and new files.
3. **Provide a sample SPA snippet** (TypeScript/`jose` library) for:
   - Generating an Ed25519 key pair.
   - Constructing the `wa.me` deep link.
   - Parsing the `#token=...&nonce=...` fragment.
