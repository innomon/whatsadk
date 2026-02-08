# ADK Go Server — JWT Authentication Guide

This document describes how to implement RS256 JWT verification on the ADK Go agent server to authenticate requests from the WhatsADK gateway.

## Token Format

The gateway sends an `Authorization: Bearer <token>` header on every request to `/run`, `/run_sse`, and session endpoints.

### JWT Header

```json
{
  "alg": "RS256",
  "typ": "JWT"
}
```

### JWT Payload (Claims)

| Claim      | Type     | Description                                      |
|------------|----------|--------------------------------------------------|
| `user_id`  | `string` | WhatsApp sender's phone number (e.g. `"919876543210"`) |
| `channel`  | `string` | Always `"whatsapp"`                               |
| `iss`      | `string` | Issuer (matches `auth.jwt.issuer` in gateway config) |
| `aud`      | `string` | Audience (matches `auth.jwt.audience` in gateway config) |
| `iat`      | `number` | Issued at (Unix timestamp)                        |
| `exp`      | `number` | Expiry (Unix timestamp, default 2 minutes from `iat`) |

### Example Decoded Token

```json
{
  "user_id": "919876543210",
  "channel": "whatsapp",
  "iss": "whatsadk-gateway",
  "aud": "adk-agent",
  "iat": 1738972800,
  "exp": 1738972920
}
```

---

## Key Setup

The gateway signs tokens with an RSA private key. The ADK server verifies using the corresponding public key.

Generate the key pair (if not already done):

```bash
openssl genrsa -out jwt_private.pem 2048
openssl rsa -in jwt_private.pem -pubout -out jwt_public.pem
```

- **Gateway** gets `jwt_private.pem`
- **ADK server** gets `jwt_public.pem`

---

## Dependencies

```bash
go get github.com/golang-jwt/jwt/v5
```

The ADK Go SDK (`google.golang.org/adk`) already depends on `github.com/gorilla/mux` for routing.

---

## JWT Verifier Package

Create a reusable `auth` package in your ADK server project.

### `auth/verifier.go`

```go
package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID  string `json:"user_id"`
	Channel string `json:"channel"`
	jwt.RegisteredClaims
}

type claimsContextKey struct{}

type JWTVerifier struct {
	publicKey *rsa.PublicKey
	issuer    string
	audience  string
}

func NewJWTVerifier(publicKeyPath, issuer, audience string) (*JWTVerifier, error) {
	keyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return &JWTVerifier{
		publicKey: rsaPub,
		issuer:    issuer,
		audience:  audience,
	}, nil
}

func (v *JWTVerifier) Verify(tokenStr string) (*Claims, error) {
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256"}),
	}
	if v.issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(v.issuer))
	}
	if v.audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.audience))
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return v.publicKey, nil
	}, parserOpts...)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.UserID == "" || claims.Channel == "" {
		return nil, fmt.Errorf("missing user_id or channel claim")
	}

	return claims, nil
}

// Middleware returns an http.Handler middleware that verifies JWT tokens
// and injects claims into the request context.
func (v *JWTVerifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := v.Verify(tokenStr)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext extracts the verified JWT claims from the request context.
func ClaimsFromContext(ctx context.Context) *Claims {
	claims, _ := ctx.Value(claimsContextKey{}).(*Claims)
	return claims
}
```

### `auth/verifier_test.go`

```go
package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestKeys(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	path := filepath.Join(t.TempDir(), "test_pub.pem")
	if err := os.WriteFile(path, pubPEM, 0644); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}

	return path, key
}

func signToken(t *testing.T, key *rsa.PrivateKey, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenStr, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenStr
}

func TestVerify_ValidToken(t *testing.T) {
	pubPath, privKey := generateTestKeys(t)

	v, err := NewJWTVerifier(pubPath, "whatsadk-gateway", "adk-agent")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	now := time.Now()
	tokenStr := signToken(t, privKey, Claims{
		UserID:  "919876543210",
		Channel: "whatsapp",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "whatsadk-gateway",
			Audience:  jwt.ClaimStrings{"adk-agent"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	})

	claims, err := v.Verify(tokenStr)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if claims.UserID != "919876543210" {
		t.Errorf("expected user_id=919876543210, got %s", claims.UserID)
	}
	if claims.Channel != "whatsapp" {
		t.Errorf("expected channel=whatsapp, got %s", claims.Channel)
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	pubPath, privKey := generateTestKeys(t)

	v, err := NewJWTVerifier(pubPath, "", "")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	past := time.Now().Add(-10 * time.Minute)
	tokenStr := signToken(t, privKey, Claims{
		UserID:  "user1",
		Channel: "whatsapp",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(past),
			ExpiresAt: jwt.NewNumericDate(past.Add(2 * time.Minute)),
		},
	})

	_, err = v.Verify(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestMiddleware_RejectsNoHeader(t *testing.T) {
	pubPath, _ := generateTestKeys(t)

	v, _ := NewJWTVerifier(pubPath, "", "")

	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/run", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_InjectsClaims(t *testing.T) {
	pubPath, privKey := generateTestKeys(t)

	v, _ := NewJWTVerifier(pubPath, "", "")

	now := time.Now()
	tokenStr := signToken(t, privKey, Claims{
		UserID:  "user42",
		Channel: "whatsapp",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	})

	var gotClaims *Claims
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/run", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotClaims == nil || gotClaims.UserID != "user42" {
		t.Errorf("expected user_id=user42 in context, got %+v", gotClaims)
	}
}
```

---

## Integration with ADK Go SDK

The ADK Go SDK uses `gorilla/mux` for routing. The key function is `adkrest.NewHandler()` which returns a standard `http.Handler`. You wrap it with the JWT middleware before passing it to the HTTP server.

### Option 1: Wrap `adkrest.NewHandler` with Middleware

This is the simplest approach — use `adkrest.NewHandler` directly and wrap it.

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/server/adkrest"
	"google.golang.org/genai"

	"yourmodule/auth"
)

func main() {
	ctx := context.Background()

	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	myAgent, err := llmagent.New(llmagent.Config{
		Name:        "my_agent",
		Model:       model,
		Description: "A helpful assistant.",
		Instruction: "You are a helpful assistant.",
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(myAgent),
	}

	// Create the ADK REST handler
	adkHandler := adkrest.NewHandler(config, 30*time.Second)

	// Create the JWT verifier
	verifier, err := auth.NewJWTVerifier(
		"secrets/jwt_public.pem",
		"whatsadk-gateway",   // must match gateway's auth.jwt.issuer
		"adk-agent",          // must match gateway's auth.jwt.audience
	)
	if err != nil {
		log.Fatalf("Failed to create JWT verifier: %v", err)
	}

	// Wrap ADK handler with JWT middleware
	handler := verifier.Middleware(adkHandler)

	log.Println("ADK server with JWT auth listening on :8000")
	log.Fatal(http.ListenAndServe(":8000", handler))
}
```

### Option 2: Use the Launcher with Middleware

If you prefer to use the ADK launcher but still want JWT auth, you can use `gorilla/mux` middleware support via `router.Use()`.

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/web"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/server/adkrest"
	"google.golang.org/genai"

	"yourmodule/auth"
)

func main() {
	ctx := context.Background()

	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	myAgent, err := llmagent.New(llmagent.Config{
		Name:        "my_agent",
		Model:       model,
		Description: "A helpful assistant.",
		Instruction: "You are a helpful assistant.",
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(myAgent),
	}

	// Create the JWT verifier
	verifier, err := auth.NewJWTVerifier(
		"secrets/jwt_public.pem",
		"whatsadk-gateway",
		"adk-agent",
	)
	if err != nil {
		log.Fatalf("Failed to create JWT verifier: %v", err)
	}

	// Build the gorilla/mux base router and apply JWT middleware
	router := web.BuildBaseRouter()
	router.Use(verifier.MiddlewareMux)

	// Mount ADK REST routes
	adkHandler := adkrest.NewHandler(config, 30*time.Second)
	router.PathPrefix("/").Handler(adkHandler)

	log.Println("ADK server with JWT auth listening on :8000")
	log.Fatal(http.ListenAndServe(":8000", router))
}
```

For gorilla/mux compatibility, add this method to the verifier:

```go
// MiddlewareMux is a gorilla/mux-compatible middleware function.
func (v *JWTVerifier) MiddlewareMux(next http.Handler) http.Handler {
	return v.Middleware(next)
}
```

### Option 3: Custom Server with Selective Auth

If you want JWT auth only on specific routes (e.g. `/run`, `/run_sse`, session endpoints) but not on health checks:

```go
package main

import (
	"log"
	"net/http"
	"time"

	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/server/adkrest"

	"yourmodule/auth"
)

func main() {
	// ... agent and config setup omitted for brevity ...

	verifier, err := auth.NewJWTVerifier(
		"secrets/jwt_public.pem",
		"whatsadk-gateway",
		"adk-agent",
	)
	if err != nil {
		log.Fatalf("Failed to create JWT verifier: %v", err)
	}

	config := &launcher.Config{
		// ... your agent loader ...
	}

	adkHandler := adkrest.NewHandler(config, 30*time.Second)

	mux := http.NewServeMux()

	// Health check — no auth
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// All ADK routes — protected by JWT
	mux.Handle("/", verifier.Middleware(adkHandler))

	log.Println("ADK server with JWT auth listening on :8000")
	log.Fatal(http.ListenAndServe(":8000", mux))
}
```

---

## Accessing Claims in ADK Callbacks

If you use ADK [callbacks](https://google.github.io/adk-docs/callbacks/) and need access to the authenticated user identity, extract claims from the request context:

```go
claims := auth.ClaimsFromContext(ctx)
if claims != nil {
	log.Printf("Request from user_id=%s, channel=%s", claims.UserID, claims.Channel)
}
```

---

## Testing the Integration

### 1. Generate a Test Token

```bash
# Install jwt-cli (or use Go)
go install github.com/golang-jwt/jwt/v5/cmd/jwt@latest

# Or generate with a quick Go script:
cat > /tmp/gen_token.go << 'EOF'
package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	keyData, _ := os.ReadFile("secrets/jwt_private.pem")
	block, _ := pem.Decode(keyData)
	key, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

	now := time.Now()
	claims := jwt.MapClaims{
		"user_id": "919876543210",
		"channel": "whatsapp",
		"iss":     "whatsadk-gateway",
		"aud":     "adk-agent",
		"iat":     now.Unix(),
		"exp":     now.Add(5 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenStr, _ := token.SignedString(key)
	fmt.Print(tokenStr)
}
EOF
TOKEN=$(go run /tmp/gen_token.go)
```

### 2. Test with `curl`

```bash
# Authenticated request
curl -X POST http://localhost:8000/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "appName": "my_agent",
    "userId": "919876543210",
    "sessionId": "919876543210",
    "newMessage": {
      "role": "user",
      "parts": [{"text": "Hello"}]
    }
  }'

# Missing auth — should return 401
curl -X POST http://localhost:8000/run \
  -H "Content-Type: application/json" \
  -d '{"appName": "my_agent", "userId": "test", "sessionId": "test", "newMessage": {"role": "user", "parts": [{"text": "Hello"}]}}'
```

### 3. Decode a Token Manually

```bash
echo "$TOKEN" | cut -d. -f2 | base64 -d 2>/dev/null | jq .
```

---

## Security Recommendations

1. **Keep TTL short** — Default is 2 minutes. Shorter TTLs limit the window if a token is intercepted.
2. **Never share the private key** — Only the gateway needs it. The ADK server only needs the public key.
3. **Validate `user_id` matches path** — On session endpoints, verify `claims.UserID` matches the `{user_id}` path parameter to prevent impersonation.
4. **Use TLS** — Always use HTTPS between the gateway and ADK server in production.
5. **Clock synchronization** — Ensure both gateway and ADK server use NTP to avoid clock skew issues with `exp`/`iat` validation.
6. **Reject on failure** — If JWT verification fails, reject the request immediately. Never fall back to unauthenticated access.
7. **File permissions** — Ensure `jwt_public.pem` is readable only by the server process (`chmod 644`), and `jwt_private.pem` is restricted (`chmod 600`).
