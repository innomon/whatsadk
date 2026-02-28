# OAuth TOTP 



### TOTP example

- Uses a **per-user random secret** (32 bytes) instead of the Ed25519 public key as the HMAC key  
  → this is the single most important security improvement
- The Ed25519 public key is now just **part of the message** (helps bind the code to the correct key pair)
- Includes **UserID**, **AppID**, and time step counter in the HMAC input
- Server checks **current window + previous & next** (±1 step) to tolerate reasonable clock drift
- Uses **constant-time comparison** for the final code check (timing-attack resistance)
- 6-digit code, 30-second step (both configurable)

### totp_generator.go  (client / authenticator side)

```go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) != 6 {
		fmt.Println("Usage (production-style client):")
		fmt.Println("  go run totp_generator.go <32-byte-secret-hex> <ed25519-pubkey-hex> <userid> <appid> <step-seconds>")
		fmt.Println("Example:")
		fmt.Println("  go run totp_generator.go 5cb40f57c2886a7b539eb78855e6746da4d9dd78fecf4852d78d4bc999492e14 3b6a27bcceb6a42d62a3a8d02a6f0d73653215771de243a63ac048a18b59da29 ashish-123 myapp 30")
		os.Exit(1)
	}

	secretHex := os.Args[1]
	pubHex    := os.Args[2]
	userID    := os.Args[3]
	appID     := os.Args[4]
	stepSec, _ := strconv.Atoi(os.Args[5])

	secret, err := hex.DecodeString(secretHex)
	if err != nil || len(secret) != 32 {
		fmt.Println("Secret must be 64 hex characters (32 bytes)")
		os.Exit(1)
	}

	pubBytes, err := hex.DecodeString(pubHex)
	if err != nil || len(pubBytes) != 32 {
		fmt.Println("Ed25519 public key must be 64 hex characters (32 bytes)")
		os.Exit(1)
	}

	if stepSec < 15 || stepSec > 120 {
		fmt.Printf("Unusual step (%ds) – continuing anyway\n", stepSec)
	}

	code := generateCode(secret, pubBytes, userID, appID, uint64(stepSec), time.Now().Unix())
	fmt.Printf("Code: %06d\n", code)
}

func generateCode(secret, pubKey []byte, userID, appID string, stepSec uint64, now int64) uint32 {
	const T0 = 0 // Unix epoch

	counter := uint64(now-T0) / stepSec

	var buf [32 + 128 + 128 + 8]byte // pub + userid + appid + counter
	n := copy(buf[:], pubKey)
	n += copy(buf[n:], userID)
	n += copy(buf[n:], appID)
	binary.BigEndian.PutUint64(buf[n:], counter)
	message := buf[:n+8]

	h := hmac.New(sha256.New, secret)
	h.Write(message)
	hash := h.Sum(nil)

	// Dynamic truncation (RFC 4226 / TOTP style)
	offset := int(hash[31] & 0x0f)
	binaryValue := binary.BigEndian.Uint32(hash[offset : offset+4])
	binaryValue &= 0x7fffffff // 31 bits

	return binaryValue % 1_000_000
}
```

### totp_verifier.go  (server side)

```go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) != 7 {
		fmt.Println("Usage (production-style server):")
		fmt.Println("  go run totp_verifier.go <32-byte-secret-hex> <ed25519-pubkey-hex> <userid> <appid> <step-seconds> <code-to-verify>")
		fmt.Println("Example:")
		fmt.Println("  go run totp_verifier.go 5cb40f57c2886a7b539eb78855e6746da4d9dd78fecf4852d78d4bc999492e14 3b6a27bcceb6a42d62a3a8d02a6f0d73653215771de243a63ac048a18b59da29 ashish-123 myapp 30 415872")
		os.Exit(1)
	}

	secretHex := os.Args[1]
	pubHex    := os.Args[2]
	userID    := os.Args[3]
	appID     := os.Args[4]
	stepSec, _ := strconv.Atoi(os.Args[5])
	codeStr   := os.Args[6]

	secret, err := hex.DecodeString(secretHex)
	if err != nil || len(secret) != 32 {
		fmt.Println("Secret must be 64 hex characters (32 bytes)")
		os.Exit(1)
	}

	pubBytes, err := hex.DecodeString(pubHex)
	if err != nil || len(pubBytes) != 32 {
		fmt.Println("Ed25519 public key must be 64 hex characters (32 bytes)")
		os.Exit(1)
	}

	userCode, err := strconv.ParseUint(codeStr, 10, 32)
	if err != nil || userCode > 999999 {
		fmt.Println("Code must be 6 digits")
		os.Exit(1)
	}

	now := time.Now().Unix()

	valid := checkCode(secret, pubBytes, userID, appID, uint64(stepSec), now, uint32(userCode))

	if valid {
		fmt.Println("VALID ✓")
	} else {
		fmt.Println("INVALID ✗")
	}
}

func checkCode(secret, pubKey []byte, userID, appID string, stepSec uint64, now int64, userCode uint32) bool {
	const T0 = 0
	const window = 1 // check current + previous + next

	counter := uint64(now-T0) / stepSec

	for delta := int64(-window); delta <= int64(window); delta++ {
		c := counter + uint64(delta)

		var buf [32 + 128 + 128 + 8]byte
		n := copy(buf[:], pubKey)
		n += copy(buf[n:], userID)
		n += copy(buf[n:], appID)
		binary.BigEndian.PutUint64(buf[n:], c)
		message := buf[:n+8]

		h := hmac.New(sha256.New, secret)
		h.Write(message)
		hash := h.Sum(nil)

		offset := int(hash[31] & 0x0f)
		binaryValue := binary.BigEndian.Uint32(hash[offset : offset+4])
		binaryValue &= 0x7fffffff

		expected := binaryValue % 1_000_000

		if subtle.ConstantTimeEq(int32(expected), int32(userCode)) == 1 {
			return true
		}
	}

	return false
}
```

### Quick production notes

| Item                        | Recommendation in real system                              |
|-----------------------------|---------------------------------------------------------------------|
| Secret                      | 32 random bytes per user, created at enrollment, never sent again |
| Storage                     | Store `bcrypt`-ed / Argon2 / PBKDF2 hash of secret (not plaintext) |
| Delivery                    | Show as QR code (Base32 encoded) during enrollment like Google Authenticator |
| Clock drift                 | Window = 1 is usually enough; 2 if many users have bad clocks      |
| Rate limiting               | 5–10 attempts per IP / per user / per minute                        |
| Replay protection           | Remember last 3–5 used counters per user (optional but strong)     |
| Rotation                    | Allow secret rotation / re-enrollment every 12–24 months           |

Let me know if you want:

- Base32 secret formatting + QR code hint
- Version that also includes device name / hardware binding
- Shorter/longer codes (8 digits)
- HOTP (counter) variant instead

```

### Quick test flow

```bash
# Generate
go run totp_generator.go 3b6a27bcceb6a42d62a3a8d02a6f0d73653215771de243a63ac048a18b59da29 ashish-123 myapp 30

# Verify (copy the number that was printed)
go run totp_verifier.go 3b6a27bcceb6a42d62a3a8d02a6f0d73653215771de243a63ac048a18b59da29 ashish-123 myapp 30 123456
```

### Security reminders (very important)

- Using the **Ed25519 public key as the HMAC key** is **only acceptable for a demonstration**.
- In a real system you should use a **per-user random secret** (32–64 bytes) that is:
  - generated securely
  - never leaves the server (or is only given to the client during enrollment)
  - stored hashed/encrypted on the server
- Replace `hmac.New(sha256.New, pubKey)` with `hmac.New(sha256.New, userSecret)` where `userSecret` is looked up by `userID`.

Let me know if you want a version with:

- separate random secret per user
- ±1 window check built-in
- constant-time comparison
- base32 secret like real TOTP apps use




