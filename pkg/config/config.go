package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level deployer configuration.
type Config struct {
	Project  string    `yaml:"project"`
	SSH      SSHConfig `yaml:"ssh"`
	Services []Service `yaml:"services"`
}

// SSHConfig holds SSH connection settings.
type SSHConfig struct {
	User     string `yaml:"user"`
	Port     int    `yaml:"port"`
	KeyPath  string `yaml:"key_path"`
	Password string `yaml:"password,omitempty"`
}

// Service defines a deployable container service.
type Service struct {
	Name        string            `yaml:"name"`
	Image       string            `yaml:"image"`
	Hosts       []string          `yaml:"hosts"`
	Ports       []string          `yaml:"ports,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Network     string            `yaml:"network,omitempty"`
	Restart     string            `yaml:"restart,omitempty"`
	Deploy      DeployConfig      `yaml:"deploy"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
}

// DeployConfig specifies deployment strategy.
type DeployConfig struct {
	Strategy  string `yaml:"strategy"`
	BatchSize int    `yaml:"batch_size"`
	Delay     string `yaml:"delay"`
}

// HealthCheckConfig specifies health check parameters.
type HealthCheckConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Type     string `yaml:"type"`
	Endpoint string `yaml:"endpoint,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Command  string `yaml:"command,omitempty"`
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout,omitempty"`
	Retries  int    `yaml:"retries"`
}

// GetInterval parses the interval string and returns a duration.
func (h HealthCheckConfig) GetInterval() time.Duration {
	d, err := time.ParseDuration(h.Interval)
	if err != nil {
		return 5 * time.Second
	}
	return d
}

// GetTimeout parses the timeout string and returns a duration.
func (h HealthCheckConfig) GetTimeout() time.Duration {
	if h.Timeout == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(h.Timeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// GetDelay parses the deploy delay string and returns a duration.
func (d DeployConfig) GetDelay() time.Duration {
	dur, err := time.ParseDuration(d.Delay)
	if err != nil {
		return 10 * time.Second
	}
	return dur
}

// LoadConfig reads and parses a YAML config file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Apply defaults
	if cfg.SSH.Port == 0 {
		cfg.SSH.Port = 22
	}
	if cfg.SSH.User == "" {
		cfg.SSH.User = "root"
	}

	for i := range cfg.Services {
		if cfg.Services[i].Deploy.Strategy == "" {
			cfg.Services[i].Deploy.Strategy = "rolling"
		}
		if cfg.Services[i].Deploy.BatchSize == 0 {
			cfg.Services[i].Deploy.BatchSize = 1
		}
		if cfg.Services[i].Restart == "" {
			cfg.Services[i].Restart = "unless-stopped"
		}
		if cfg.Services[i].HealthCheck.Retries == 0 {
			cfg.Services[i].HealthCheck.Retries = 3
		}
		if cfg.Services[i].HealthCheck.Interval == "" {
			cfg.Services[i].HealthCheck.Interval = "5s"
		}
	}

	return &cfg, nil
}
