package config

import (
	"flag"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	WhatsApp WhatsAppConfig `yaml:"whatsapp"`
	Agent    AgentConfig    `yaml:"agent"`
}

type WhatsAppConfig struct {
	StorePath string `yaml:"store_path"`
	LogLevel  string `yaml:"log_level"`
}

type AgentConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Instruction string `yaml:"instruction"`
	Model       string `yaml:"model"`
	APIKey      string `yaml:"api_key"`
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
	if c.Agent.Name == "" {
		c.Agent.Name = "whatsapp_assistant"
	}
	if c.Agent.Description == "" {
		c.Agent.Description = "A helpful WhatsApp assistant powered by AI"
	}
	if c.Agent.Instruction == "" {
		c.Agent.Instruction = "You are a helpful assistant responding via WhatsApp. Be concise and friendly."
	}
	if c.Agent.Model == "" {
		c.Agent.Model = "gemini-2.5-flash"
	}
}

func (c *Config) applyEnvOverrides() {
	if apiKey := os.Getenv("GOOGLE_API_KEY"); apiKey != "" {
		c.Agent.APIKey = apiKey
	}
}
