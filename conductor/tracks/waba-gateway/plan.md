# WABA (WhatsApp Business API) Gateway Implementation Plan

## Objective
Implement a new entry point and service package to support the official WhatsApp Cloud API (WABA) for Business accounts, enabling seamless integration with the ADK agent without disrupting the existing `whatsmeow`-based gateway.

## Scope & Impact
- **Isolation:** The new `waba-gateway` will be a separate binary (`cmd/waba-gateway`) ensuring zero impact on the existing `cmd/gateway`.
- **Reusability:** It will leverage existing internal packages: `config`, `agent` (for ADK communication), and `store` (for database persistence).
- **New Package:** A new `internal/waba` package will be introduced to handle Meta's specific webhook verification, signature validation, and API requests.

## Proposed Solution

### 1. Configuration Updates (`internal/config/config.go`)
Extend the existing configuration structure to include WABA credentials securely.

```go
type Config struct {
	// ... existing fields ...
	WABA WABAConfig `yaml:"waba"`
}

type WABAConfig struct {
	Enabled           bool   `yaml:"enabled"`
	Port              int    `yaml:"port"` // Webhook listener port
	AppSecret         string `yaml:"app_secret"`
	VerifyToken       string `yaml:"verify_token"`
	AccessToken       string `yaml:"access_token"`
	PhoneNumberID     string `yaml:"phone_number_id"`
	BusinessAccountID string `yaml:"business_account_id"`
}
```
*Note: Update `applyDefaults` and `applyEnvOverrides` to handle WABA variables.*

### 2. Core WABA Logic (`internal/waba`)
Create a new package to encapsulate Meta Graph API interactions.

- **`internal/waba/client.go`**:
  - Implement a client to send messages (text, media) via the `https://graph.facebook.com/v19.0/{Phone-Number-ID}/messages` endpoint.
  - Handle rate limiting and error responses from Meta.

- **`internal/waba/webhook.go`**:
  - Implement HTTP handlers for Meta's webhook endpoints.
  - **GET Handler**: Handle the `hub.challenge` verification during webhook setup.
  - **POST Handler**: Receive incoming messages and statuses.
  - Implement `X-Hub-Signature-256` validation using the `AppSecret` to ensure payloads are authentically from Meta.

### 3. Application Entry Point (`cmd/waba-gateway/main.go`)
Create the new executable that wires everything together.

- Load configuration and initialize the database store.
- Initialize the ADK `agent.Client`.
- Initialize the `waba.Client` and `waba.WebhookHandler`.
- Start an HTTP server listening on the configured port for incoming webhooks.
- Route incoming WABA text and media messages to the ADK `agent.Client.ChatParts` method, similar to how the `whatsmeow` client does it.
- Route ADK responses back through the `waba.Client` to the user.

### 4. Database & Logging Integration
- Ensure the WABA gateway logs incoming and outgoing messages to the existing `store.Store` (PostgreSQL) using the same conventions so MCP tools (`get_recent_messages`, etc.) continue to work seamlessly.

### 5. Documentation Updates
- Update `README.md` and `ARCHITECTURE.md` to reflect the addition of the `waba-gateway` binary, explaining its purpose alongside the original `gateway`.
- Add documentation on how to configure the WABA environment variables and set up the Meta App webhook.

## Verification
- Add unit tests for the webhook signature validation in `internal/waba/webhook_test.go`.
- Add unit tests for parsing the complex Meta webhook JSON structure.
- Provide a clear instruction set or script on how to test the integration using the Meta Developer Test environment.
