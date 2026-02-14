package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/innomon/whatsadk/internal/config"
)

type KeyRegistry struct {
	appKeys map[string]*rsa.PublicKey
}

func NewKeyRegistry(apps map[string]config.AppVerifyConfig) (*KeyRegistry, error) {
	registry := &KeyRegistry{
		appKeys: make(map[string]*rsa.PublicKey, len(apps)),
	}

	for appName, appCfg := range apps {
		key, err := loadPublicKey(appCfg.PublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load public key for app %q: %w", appName, err)
		}
		registry.appKeys[appName] = key
	}

	return registry, nil
}

func (r *KeyRegistry) GetAppPublicKey(appName string) (*rsa.PublicKey, error) {
	key, ok := r.appKeys[appName]
	if !ok {
		return nil, fmt.Errorf("unknown app: %s", appName)
	}
	return key, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in key data")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA")
	}

	return rsaPub, nil
}
