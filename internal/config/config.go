package config

import (
	"flag"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	WhatsApp WhatsAppConfig `yaml:"whatsapp"`
	ADK      ADKConfig      `yaml:"adk"`
	Auth     AuthConfig     `yaml:"auth"`
}

type AuthConfig struct {
	JWT JWTConfig `yaml:"jwt"`
}

type JWTConfig struct {
	PrivateKeyPath string `yaml:"private_key_path"`
	Issuer         string `yaml:"issuer,omitempty"`
	Audience       string `yaml:"audience,omitempty"`
	TTL            string `yaml:"ttl,omitempty"`
}

type WhatsAppConfig struct {
	StorePath        string   `yaml:"store_path"`
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
	if c.WhatsApp.StorePath == "" {
		c.WhatsApp.StorePath = "whatsapp.db"
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
}
