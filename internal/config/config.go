package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration for the proxy.
type Config struct {
	// Required
	Endpoint string `mapstructure:"endpoint"`
	Token    string `mapstructure:"token"`

	// Server
	Port       int    `mapstructure:"proxy_port"`
	ListenAddr string `mapstructure:"listen_addr"`
	BindAll    bool   `mapstructure:"bind_all"`

	// Translation / mock
	MockVersion string `mapstructure:"mock_version"`
	APIPrefix   string `mapstructure:"api_prefix"`

	// HTTP client
	Timeout time.Duration `mapstructure:"timeout"`

	// Security
	RateLimit   float64 `mapstructure:"rate_limit"`
	TLSCert     string  `mapstructure:"tls_cert"`
	TLSKey      string  `mapstructure:"tls_key"`
	MaxBodySize int64   `mapstructure:"max_body_size"`

	// Logging
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`
	NoColor   bool   `mapstructure:"no_color"`
}

// ListenAddress returns the fully qualified listen address (host:port).
func (c *Config) ListenAddress() string {
	addr := c.ListenAddr
	if c.BindAll {
		addr = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%d", addr, c.Port)
}

// UpstreamURL constructs a full upstream URL for the given path segment.
// Example: UpstreamURL("/models") → "https://owui.example.com/api/models"
func (c *Config) UpstreamURL(path string) string {
	base := strings.TrimRight(c.Endpoint, "/")
	prefix := strings.TrimRight(c.APIPrefix, "/")
	return fmt.Sprintf("%s%s%s", base, prefix, path)
}

// Load reads configuration from viper (flags + env) and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		Endpoint:    viper.GetString("endpoint"),
		Token:       viper.GetString("token"),
		Port:        viper.GetInt("proxy_port"),
		ListenAddr:  viper.GetString("listen_addr"),
		BindAll:     viper.GetBool("bind_all"),
		MockVersion: viper.GetString("mock_version"),
		APIPrefix:   viper.GetString("api_prefix"),
		Timeout:     viper.GetDuration("timeout"),
		RateLimit:   viper.GetFloat64("rate_limit"),
		TLSCert:     viper.GetString("tls_cert"),
		TLSKey:      viper.GetString("tls_key"),
		MaxBodySize: viper.GetInt64("max_body_size"),
		LogLevel:    viper.GetString("log_level"),
		LogFormat:   viper.GetString("log_format"),
		NoColor:     viper.GetBool("no_color"),
	}

	// Apply defaults for unset values.
	if cfg.Port == 0 {
		cfg.Port = 11434
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1"
	}
	if cfg.MockVersion == "" {
		cfg.MockVersion = "0.6.5"
	}
	if cfg.APIPrefix == "" {
		cfg.APIPrefix = "/api"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 300 * time.Second
	}
	if cfg.MaxBodySize == 0 {
		cfg.MaxBodySize = 100 * 1024 * 1024
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = "text"
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("--endpoint or OWUI_ENDPOINT is required")
	}
	if !strings.HasPrefix(c.Endpoint, "http://") && !strings.HasPrefix(c.Endpoint, "https://") {
		return fmt.Errorf("--endpoint must start with http:// or https://")
	}
	if c.Token == "" {
		return fmt.Errorf("--token or OWUI_TOKEN is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("--port must be between 1 and 65535")
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return fmt.Errorf("both --tls-cert and --tls-key must be provided together")
	}
	if c.RateLimit < 0 {
		return fmt.Errorf("--rate-limit must be >= 0")
	}
	return nil
}
