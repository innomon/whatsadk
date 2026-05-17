# Implementation Guide: WhatsApp Authentication (Reverse OTP & TOTP)

This guide provides instructions for implementing two powerful WhatsApp-based authentication flows in your application: **Reverse OTP (Mobile-Originated Verification)** and **TOTP (Time-based One-Time Password)**.

---

## 1. Reverse OTP (Mobile-Originated Verification)

This flow allows users to verify their phone number by sending a message *from* their WhatsApp account to your gateway.

### High-Level Flow
1. **User Request:** User enters their phone number in your Web/Mobile app (PWA).
2. **Backend Token Generation:** Your backend generates an **RS256 JWT** containing the user's phone number and a unique challenge ID.
3. **Deep Link:** Your app presents a WhatsApp deep link (e.g., `https://wa.me/<gateway_number>?text=<jwt>`).
4. **User Action:** User clicks the link and sends the pre-filled JWT message to the WhatsApp Gateway.
5. **Gateway Verification:** The Gateway verifies the JWT's signature (using your public key) and ensures the sender's phone number matches the one in the JWT.
6. **Callback & OTP:** The Gateway calls your backend's `/callback` endpoint with a Gateway-signed JWT. Your backend returns a 4-6 digit OTP.
7. **Relay:** The Gateway sends the OTP to the user via a WhatsApp reply.
8. **Final Verification:** The user enters the OTP in your app's login screen.

### Implementation Requirements

#### Backend (Your Project)
- **Keys:** Generate an RSA key pair. Provide the public key to the Gateway.
- **JWT Generation:** Use RS256.
  - **Claims:** `mobile` (E.164 format), `app_name` (identifier), `challenge_id` (UUID), `iat`, `exp` (short-lived, e.g., 5 min).
- **Callback Endpoint:** `POST /api/v1/auth/whatsapp/callback?challenge_id=...`
  - **Verification:** Verify the Gateway's signature using the Gateway's public key.
  - **Match:** Ensure `challenge_id` in JWT matches the query parameter.
  - **Response:** `{"otp": "123456"}`.

#### Gateway (WhatsADK)
- **Configuration:** Add your app's public key and callback URL to `config.yaml`.
- **Handling:** Gateway detects incoming JWTs, validates them, and performs the callback loop.

---

## 2. TOTP (Time-based One-Time Password)

This flow provides a device-bound, 6-digit code for 2FA, optimized for delivery via WhatsApp or app-to-app communication.

### Key Security Principles
- **Algorithm:** TOTP (RFC 6238) using HMAC-SHA256.
- **Key Binding:** Ed25519 (EdDSA) public keys are used to bind the TOTP to a specific device.
- **Secrets:** Use a per-user random 32-byte secret (HMAC key), *not* the public key itself.

### Implementation Steps

#### A. Enrollment (Setup)
1. **Key Generation:** Client generates an Ed25519 key pair.
2. **Registration:** Client sends the Public Key to the Server.
3. **Secret Generation:** Server generates a random 32-byte secret for the user and stores it securely (encrypted/hashed).
4. **Shared Secret:** Server provides the secret to the Client (e.g., via QR code or secure channel).

#### B. Generation (Client Side)
Use the following logic to generate the 6-digit code:
```go
func generateCode(secret, pubKey []byte, userID, appID string, stepSec uint64) uint32 {
    counter := uint64(time.Now().Unix()) / stepSec
    
    // Message binding: Public Key + UserID + AppID + Counter
    var buf [32 + 128 + 128 + 8]byte
    n := copy(buf[:], pubKey)
    n += copy(buf[n:], userID)
    n += copy(buf[n:], appID)
    binary.BigEndian.PutUint64(buf[n:], counter)
    
    h := hmac.New(sha256.New, secret)
    h.Write(buf[:n+8])
    hash := h.Sum(nil)

    // Dynamic Truncation
    offset := int(hash[31] & 0x0f)
    binaryValue := binary.BigEndian.Uint32(hash[offset : offset+4]) & 0x7fffffff
    return binaryValue % 1_000_000
}
```

#### C. Verification (Server Side)
- **Window Check:** Always check the current counter value and ±1 window to account for clock drift.
- **Constant-Time Comparison:** Use `subtle.ConstantTimeEq` to prevent timing attacks.
- **Rate Limiting:** Enforce strict limits (e.g., 5 attempts/minute).

---

## 3. Security Checklist

| Feature | Best Practice |
| :--- | :--- |
| **Algorithm** | Use **RS256** for Reverse OTP (standard) and **EdDSA** for TOTP (compactness). |
| **Phone Format** | Use **E.164 digits** (e.g., `910987654321`) consistently. |
| **Secrets** | Never hardcode keys. Store secrets hashed or in a secure Vault. |
| **Callbacks** | Only use **HTTPS**. Derrive callback URLs from static configuration, never from untrusted JWT claims. |
| **SSRF** | Validate callback destinations strictly. |
| **Clock Drift** | Use a 30-second step for TOTP with a window of ±1. |

---

## 4. References & Tools

- **WhatsADK Docs:** Refer to [whatsapp-oauth.md](https://github.com/innomon/whatsadk/blob/main/docs/whatsapp-oauth.md) and [totp.md](https://github.com/innomon/whatsadk/blob/main/docs/totp.md) in the `whatsadk` repository.
- **Deep Link Generator:** `https://wa.me/<number>?text=<encoded_jwt>`
- **JWT Libraries:** `golang-jwt/jwt/v5` (Go), `jsonwebtoken` (Node.js).
