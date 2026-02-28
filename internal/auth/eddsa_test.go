package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEdDSAKey_PEM(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalPKCS8: %v", err)
	}

	dir := t.TempDir()
	pemPath := filepath.Join(dir, "key.pem")
	f, err := os.Create(pemPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: der}); err != nil {
		t.Fatalf("pem.Encode: %v", err)
	}
	f.Close()

	loaded, err := LoadEdDSAKey(pemPath)
	if err != nil {
		t.Fatalf("LoadEdDSAKey: %v", err)
	}

	if !priv.Equal(loaded) {
		t.Fatal("loaded key does not match original")
	}
}

func TestLoadEdDSAKey_RawSeed(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	dir := t.TempDir()
	seedPath := filepath.Join(dir, "seed.bin")
	if err := os.WriteFile(seedPath, priv.Seed(), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadEdDSAKey(seedPath)
	if err != nil {
		t.Fatalf("LoadEdDSAKey: %v", err)
	}

	if !priv.Equal(loaded) {
		t.Fatal("loaded key does not match original")
	}
}

func TestLoadEdDSAKey_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.key")
	if err := os.WriteFile(badPath, []byte("not a key"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadEdDSAKey(badPath)
	if err == nil {
		t.Fatal("expected error for invalid key file")
	}
}

func TestEdDSAPublicKeyBase64(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	b64 := EdDSAPublicKeyBase64(priv)
	decoded, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		t.Fatalf("expected %d bytes, got %d", ed25519.PublicKeySize, len(decoded))
	}

	pub := priv.Public().(ed25519.PublicKey)
	if !pub.Equal(ed25519.PublicKey(decoded)) {
		t.Fatal("decoded public key does not match")
	}
}
