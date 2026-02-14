package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/innomon/whatsadk/internal/config"
)

func generateTestPublicKeyFile(t *testing.T) (string, *rsa.PrivateKey) {
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

	path := filepath.Join(t.TempDir(), "public.pem")
	if err := os.WriteFile(path, pubPEM, 0644); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}

	return path, key
}

func TestKeyRegistry_LoadKeys(t *testing.T) {
	pubPath, _ := generateTestPublicKeyFile(t)

	apps := map[string]config.AppVerifyConfig{
		"test-app": {PublicKeyPath: pubPath},
	}

	registry, err := NewKeyRegistry(apps)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	key, err := registry.GetAppPublicKey("test-app")
	if err != nil {
		t.Fatalf("failed to get key: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
}

func TestKeyRegistry_MissingKey(t *testing.T) {
	apps := map[string]config.AppVerifyConfig{
		"test-app": {PublicKeyPath: "/nonexistent/public.pem"},
	}

	_, err := NewKeyRegistry(apps)
	if err == nil {
		t.Fatal("expected error for missing key file")
	}
}

func TestKeyRegistry_UnknownApp(t *testing.T) {
	pubPath, _ := generateTestPublicKeyFile(t)

	apps := map[string]config.AppVerifyConfig{
		"test-app": {PublicKeyPath: pubPath},
	}

	registry, err := NewKeyRegistry(apps)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	_, err = registry.GetAppPublicKey("unknown-app")
	if err == nil {
		t.Fatal("expected error for unknown app")
	}
}
