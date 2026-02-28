# WhatsApp OAuth Implementation Plan (NATS NKeys + JWT)

## Phase 1: Dependencies & Core Auth
1. **Add NATS dependencies:**
   - `github.com/nats-io/nkeys`
   - `github.com/nats-io/jwt/v2`
2. **Implement `internal/auth/nkeys.go`:**
   - Helper to load the Gateway's Account NKey from a file or environment.
   - Utility to verify NKey signatures.
3. **Implement `internal/auth/jwt_nats.go`:**
   - Function to create and sign a NATS User JWT using the Account NKey.
   - Claims should include the user's phone number as the subject (`sub`).

## Phase 2: Configuration Update
1. **Update `internal/config/config.go`:**
   - Add `OAuth` section:
     ```yaml
     oauth:
       enabled: true
       account_nkey_path: "secrets/account.nk"
       spa_url: "https://chat.myadk.app"
       issuer_name: "My WhatsApp Gateway"
     ```
2. **Update `config/config.yaml`** with default values.

## Phase 3: WhatsApp Command Handling
1. **Create `internal/auth/oauth_handler.go`:**
   - `Handle(senderPhone, messageBody string) (string, error)`
   - Detects `AUTH <UserPubKey> <nonce>`.
   - Validates the request.
   - Returns a WhatsApp message with the deep link: `<SPA_URL>/auth#token=<JWT>&nonce=<nonce>`.
2. **Update `internal/whatsapp/client.go`:**
   - Add `oauthHandler` field to `Client` struct.
   - In `handleMessage`, check for OAuth commands before forwarding to the ADK agent.
   - If a message starts with `AUTH`, pass it to `oauthHandler`.

## Phase 4: Integration & Startup
1. **Update `cmd/gateway/main.go`:**
   - Initialize the `oauthHandler` if enabled in config.
   - Load the Gateway's Account NKey.
   - Pass the handler to the WhatsApp `Client`.

## Phase 5: Testing & Validation
1. **Unit Tests:**
   - Test NATS JWT generation and signature verification.
   - Test `oauthHandler` command parsing and link generation.
2. **Integration Test:**
   - Simulate a WhatsApp message with a mock NKey and verify the generated JWT contains the correct claims.
3. **Manual E2E:**
   - Use a test WhatsApp number to send an `AUTH` message.
   - Verify the returned link is correct and the JWT is valid using `jwt.io` or a NATS tool.

## Phase 6: Documentation & Examples
1. **Update `README.md`** with instructions on how to use WhatsApp Login.
2. **Provide a sample SPA snippet** (JavaScript) for generating NKeys and handling the `#token` fragment.
