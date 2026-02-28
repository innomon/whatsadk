package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/innomon/whatsadk/internal/auth"
)

func main() {
	outPath := flag.String("out", "secrets/oauth_ed25519.pem", "output path for the Ed25519 private key PEM file")
	flag.Parse()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate Ed25519 key pair: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatalf("Failed to marshal private key: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o700); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	f, err := os.OpenFile(*outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Fatalf("Failed to create key file: %v", err)
	}
	defer f.Close()

	if err := pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: der}); err != nil {
		log.Fatalf("Failed to write PEM: %v", err)
	}

	_ = pub // used via auth helper below
	fmt.Printf("✅ Ed25519 private key written to: %s\n", *outPath)
	fmt.Printf("📋 Public key (base64url, share with ADK server):\n   %s\n", auth.EdDSAPublicKeyBase64(priv))
}
