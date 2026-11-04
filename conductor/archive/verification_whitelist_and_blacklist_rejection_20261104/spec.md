# Specification: Verification Database Auto-Whitelisting & Blacklist Explicit Rejection

## 1. Overview
Currently:
1. When a phone number successfully completes verification (HMAC or RSA), it is not persisted as a whitelisted user in the database. As a result, subsequent interactions still rely strictly on static `whitelisted_users` config or default country filters.
2. In `internal/whatsapp/client.go#handleMessage`, incoming messages from blacklisted numbers are silently dropped at the very start of the message handler before reaching `verifyHandler`. Therefore, a blacklisted user attempting verification receives no feedback explaining why their request was rejected.

This track introduces:
- A persistent `whitelisted_numbers` store capability across SQLite-P2P, PostgreSQL, and SurrealDB.
- Automatic insertion of successfully verified numbers into `whitelisted_numbers`.
- Dynamic whitelist checking in `isUserAllowed(jid)` against `store.IsWhitelisted(ctx, phone)`.
- Blacklist verification intercept: when a blacklisted number sends a verification token (`VERIFY ...` or RSA JWT `eyJ...`), send an explicit rejection message (`config.messages.blacklisted`) to the user instead of silently dropping the message.

## 2. Requirements

### 2.1 Persistent Whitelisted Numbers in Store (`internal/store`)
Implement `whitelisted_numbers` table/view analogous to `blacklisted_numbers`:
1. **Schema**:
   - `phone` (VARCHAR/TEXT, Primary Key / Unique)
   - `reason` (TEXT)
   - `created_at` (TIMESTAMPTZ / DATETIME)
2. **Backends**:
   - **PostgreSQL / SQL Store**: `CREATE TABLE IF NOT EXISTS whitelisted_numbers ...` with `IsWhitelisted`, `AddWhitelist`, `RemoveWhitelist`, `ListWhitelist`.
   - **SQLite-P2P**: `CREATE VIEW IF NOT EXISTS whitelisted_numbers AS SELECT ... FROM crm_store WHERE entity = 'whitelisted_numbers'`. P2P CRUD operations implemented in `p2p_whitelist.go`.
   - **SurrealDB**: `whitelisted_numbers:<phone>` record management.
3. **Public API on `store.Store`**:
   - `IsWhitelisted(ctx context.Context, phone string) (bool, error)`
   - `AddWhitelist(ctx context.Context, phone, reason string) error`
   - `RemoveWhitelist(ctx context.Context, phone string) error`
   - `ListWhitelist(ctx context.Context) ([]WhitelistedNumber, error)`

### 2.2 Explicit Blacklist Rejection for Verification Attempts
In `internal/whatsapp/client.go#handleMessage`:
1. Inspect message text. If `auth.IsVerificationToken(text) != nil || verification.IsHMACVerificationMessage(text)`:
   - Check if user is blacklisted.
   - If blacklisted, send `c.cfg.Verification.Messages.Blacklisted` (or fallback `"🚫 This number has been blocked."`) as a direct WhatsApp response, audit the rejection, and exit.
2. Only silently drop non-verification messages if the user is blacklisted.

### 2.3 Auto-Whitelisting on Successful Verification
In `internal/whatsapp/client.go#handleMessage`:
1. When `c.verifyHandler.Handle(ctx, userID, text)` returns a successful response (i.e. starts with `appCfg.SuccessMessage` or `messages.Success` / OTP delivery, not error/expired/blacklisted/empty):
   - Call `c.store.AddWhitelist(ctx, userID, "verified via "+appID)` (or "verified").
   - Log info that user was added to database whitelist.

### 2.4 Dynamic Whitelist Check in Access Control
In `internal/whatsapp/client.go#isUserAllowed`:
1. Check `c.cfg.IsUserWhitelisted(phone)`.
2. If not found in config whitelist and `c.store != nil`:
   - Query `c.store.IsWhitelisted(ctx, phone)`. If true, allow user.
