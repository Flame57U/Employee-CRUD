package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// BrokerConfig holds broker REST API credentials and connection options.
// All fields are loaded from config.agent.yaml and never transmitted to SaaS.
type BrokerConfig struct {
	Name      string `yaml:"name"`
	APIKey    string `yaml:"api_key"`
	SecretKey string `yaml:"secret_key"`
	// TradePassword is the secondary auth credential required by some brokers
	// (e.g. Huatai, CITIC) in addition to the API key pair.
	TradePassword string `yaml:"trade_password"`
	// Simulated connects to the broker's paper-trading environment when true.
	Simulated bool `yaml:"simulated"`
}

// AgentConfig is the top-level configuration for the LocalAgent binary.
type AgentConfig struct {
	SaaSURL  string       `yaml:"saas_url"`
	Email    string       `yaml:"email"`
	Password string       `yaml:"password"`
	Broker   BrokerConfig `yaml:"broker"`
}

// Load reads and parses the YAML config file at path.
func Load(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.SaaSURL == "" || cfg.Email == "" || cfg.Password == "" {
		return nil, fmt.Errorf("config missing required fields: saas_url, email, password")
	}
	if cfg.Broker.APIKey == "" || cfg.Broker.SecretKey == "" {
		return nil, fmt.Errorf("config missing broker.api_key or broker.secret_key")
	}
	return &cfg, nil
}
