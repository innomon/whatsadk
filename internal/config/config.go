package config

import (
	"flag"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	WhatsApp     WhatsAppConfig     `yaml:"whatsapp"`
	ADK          ADKConfig          `yaml:"adk"`
	Auth         AuthConfig         `yaml:"auth"`
	Verification VerificationConfig `yaml:"verification"`
}

type VerificationConfig struct {
	Enabled         bool                      `yaml:"enabled"`
	CallbackTimeout string                    `yaml:"callback_timeout"`
	DatabaseURL     string                    `yaml:"database_url"`
	DevOpsNumbers   []string                  `yaml:"devops_numbers"`
	Apps            map[string]AppVerifyConfig `yaml:"apps"`
	Messages        VerificationMessages       `yaml:"messages"`
}

type AppVerifyConfig struct {
	PublicKeyPath string `yaml:"public_key_path"`
}

type VerificationMessages struct {
	Success       string `yaml:"success"`
	Expired       string `yaml:"expired"`
	PhoneMismatch string `yaml:"phone_mismatch"`
	Blacklisted   string `yaml:"blacklisted"`
	Error         string `yaml:"error"`
}

type AuthConfig struct {
	JWT   JWTConfig   `yaml:"jwt"`
	OAuth OAuthConfig `yaml:"oauth"`
}

type OAuthConfig struct {
	Enabled   bool   `yaml:"enabled"`
	KeyPath   string `yaml:"key_path"`
	SPAURL    string `yaml:"spa_url"`
	Issuer    string `yaml:"issuer"`
	Audience  string `yaml:"audience"`
	TTL       string `yaml:"ttl"`
	RateLimit int    `yaml:"rate_limit"`
}

type JWTConfig struct {
	PrivateKeyPath string `yaml:"private_key_path"`
	Issuer         string `yaml:"issuer,omitempty"`
	Audience       string `yaml:"audience,omitempty"`
	TTL            string `yaml:"ttl,omitempty"`
}

type WhatsAppConfig struct {
	StoreDSN         string   `yaml:"store_dsn"`
	LogLevel         string   `yaml:"log_level"`
	WhitelistedUsers []string `yaml:"whitelisted_users"`
}

type ADKConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AppName   string `yaml:"app_name"`
	Streaming bool   `yaml:"streaming"`
	APIKey    string `yaml:"api_key"`
}

func Load() (*Config, error) {
	configPath := findConfigPath()
	if configPath == "" {
		return defaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()
	cfg.applyEnvOverrides()
	return &cfg, nil
}

func findConfigPath() string {
	var configArg string
	flag.StringVar(&configArg, "config", "", "path to config file")
	flag.Parse()
	if configArg != "" {
		return configArg
	}

	if envPath := os.Getenv("CONFIG_FILE"); envPath != "" {
		return envPath
	}

	searchPaths := []string{
		"config.yaml",
		"config/config.yaml",
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		searchPaths = append(searchPaths,
			filepath.Join(exeDir, "config.yaml"),
			filepath.Join(exeDir, "config", "config.yaml"),
		)
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func defaultConfig() *Config {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.applyEnvOverrides()
	return cfg
}

func (c *Config) applyDefaults() {
	if c.WhatsApp.StoreDSN == "" {
		c.WhatsApp.StoreDSN = "postgres://localhost:5432/whatsadk?sslmode=disable"
	}
	if c.WhatsApp.LogLevel == "" {
		c.WhatsApp.LogLevel = "INFO"
	}
	if c.ADK.Endpoint == "" {
		c.ADK.Endpoint = "http://localhost:8000"
	}
	if c.ADK.AppName == "" {
		c.ADK.AppName = "my_agent"
	}
	if c.Verification.Messages.Success == "" {
		c.Verification.Messages.Success = "✅ Verification successful! You can now return to the app."
	}
	if c.Verification.Messages.Expired == "" {
		c.Verification.Messages.Expired = "❌ Verification failed. The link may have expired. Please request a new one from the app."
	}
	if c.Verification.Messages.PhoneMismatch == "" {
		c.Verification.Messages.PhoneMismatch = "❌ Verification failed. Please make sure you're sending from the same number you registered with."
	}
	if c.Verification.Messages.Blacklisted == "" {
		c.Verification.Messages.Blacklisted = "🚫 This number has been blocked from verification."
	}
	if c.Verification.DatabaseURL == "" {
		c.Verification.DatabaseURL = "postgres://localhost:5432/whatsadk?sslmode=disable"
	}
	if c.Verification.Messages.Error == "" {
		c.Verification.Messages.Error = "⚠️ Something went wrong. Please try again in a moment."
	}
	if c.Verification.CallbackTimeout == "" {
		c.Verification.CallbackTimeout = "10s"
	}
	if c.Auth.OAuth.Issuer == "" {
		c.Auth.OAuth.Issuer = "whatsadk-gateway"
	}
	if c.Auth.OAuth.TTL == "" {
		c.Auth.OAuth.TTL = "24h"
	}
	if c.Auth.OAuth.RateLimit == 0 {
		c.Auth.OAuth.RateLimit = 5
	}
}

func (c *Config) IsUserWhitelisted(userID string) bool {
	for _, u := range c.WhatsApp.WhitelistedUsers {
		if u == userID {
			return true
		}
	}
	return false
}

func (c *Config) applyEnvOverrides() {
	if endpoint := os.Getenv("ADK_ENDPOINT"); endpoint != "" {
		c.ADK.Endpoint = endpoint
	}
	if appName := os.Getenv("ADK_APP_NAME"); appName != "" {
		c.ADK.AppName = appName
	}
	if apiKey := os.Getenv("ADK_API_KEY"); apiKey != "" {
		c.ADK.APIKey = apiKey
	}
	if keyPath := os.Getenv("AUTH_JWT_PRIVATE_KEY_PATH"); keyPath != "" {
		c.Auth.JWT.PrivateKeyPath = keyPath
	}
	if v := os.Getenv("VERIFICATION_ENABLED"); v == "true" {
		c.Verification.Enabled = true
	}
	if v := os.Getenv("VERIFICATION_CALLBACK_TIMEOUT"); v != "" {
		c.Verification.CallbackTimeout = v
	}
	if v := os.Getenv("VERIFICATION_DATABASE_URL"); v != "" {
		c.Verification.DatabaseURL = v
	}
	if v := os.Getenv("WHATSAPP_STORE_DSN"); v != "" {
		c.WhatsApp.StoreDSN = v
	}
	if v := os.Getenv("OAUTH_ENABLED"); v == "true" {
		c.Auth.OAuth.Enabled = true
	}
	if v := os.Getenv("OAUTH_KEY_PATH"); v != "" {
		c.Auth.OAuth.KeyPath = v
	}
	if v := os.Getenv("OAUTH_SPA_URL"); v != "" {
		c.Auth.OAuth.SPAURL = v
	}
}
