package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// UpstreamRoute represents a routing rule for upstream services
type UpstreamRoute struct {
	Path            string `json:"path" yaml:"path"`                         // URL path prefix to match
	UpstreamURL     string `json:"upstream_url" yaml:"upstream_url"`         // Target upstream URL
	StripPath       bool   `json:"strip_path" yaml:"strip_path"`             // Whether to strip the path prefix when forwarding
	EnableWebSocket bool   `json:"enable_websocket" yaml:"enable_websocket"` // Whether to enable WebSocket proxying for this route
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port string `yaml:"port"`
	Host string `yaml:"host"`
}

// TLSConfig holds TLS-specific configuration
type TLSConfig struct {
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

// OIDCConfig holds OpenID Connect configuration
type OIDCConfig struct {
	ProviderURL  string `yaml:"provider_url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
	Scopes       string `yaml:"scopes"`
}

// SessionConfig holds session management configuration
type SessionConfig struct {
	Secret     string `yaml:"secret"`
	CookieName string `yaml:"cookie_name"`
	MaxAge     int    `yaml:"max_age"`
}

// ProxyConfig holds proxy-specific configuration
type ProxyConfig struct {
	Routes []UpstreamRoute `yaml:"routes"`
}

// SecurityConfig holds security-specific configuration
type SecurityConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	Enabled        bool `yaml:"enabled"`
	CheckUpstreams bool `yaml:"check_upstreams"`
}

// YAMLConfig represents the YAML configuration structure
type YAMLConfig struct {
	Server   ServerConfig   `yaml:"server"`
	TLS      TLSConfig      `yaml:"tls"`
	OIDC     OIDCConfig     `yaml:"oidc"`
	Session  SessionConfig  `yaml:"session"`
	Proxy    ProxyConfig    `yaml:"proxy"`
	Security SecurityConfig `yaml:"security"`
	Logging  LoggingConfig  `yaml:"logging"`
	Health   HealthConfig   `yaml:"health"`
}

// Config holds the application configuration (internal representation)
type Config struct {
	// Server configuration
	Port string
	Host string

	// OpenID Connect configuration
	OIDCProviderURL  string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCScopes       []string

	// Proxy configuration
	UpstreamRoutes    []UpstreamRoute // Multi-upstream configuration
	SessionSecret     string
	SessionCookieName string
	SessionMaxAge     int

	// Security configuration
	AllowedOrigins     []string
	TLSCertFile        string
	TLSKeyFile         string
	InsecureSkipVerify bool

	// Logging configuration
	LogLevel  string
	LogFormat string

	// Health configuration
	HealthEnabled        bool
	HealthCheckUpstreams bool
}

// LoadConfig loads configuration from YAML file
func LoadConfig() (*Config, error) {
	// Try CONFIG_FILE environment variable first, then default to config.yaml
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.yaml"
		// Check if the default config file exists
		if _, err := os.Stat(configFile); err != nil {
			return nil, fmt.Errorf("CONFIG_FILE environment variable not set and default config.yaml not found. Please set CONFIG_FILE or create config.yaml")
		}
	}

	config, err := LoadFromYAML(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load YAML configuration from %s: %v", configFile, err)
	}

	// Allow environment variables to override sensitive values
	config.applyEnvironmentOverrides()

	// Validate the final configuration
	if err := config.validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// LoadFromYAML loads configuration from a YAML file
func LoadFromYAML(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", filepath, err)
	}

	var yamlConfig YAMLConfig
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %v", err)
	}

	// Convert YAML config to internal Config structure
	config := &Config{
		Port:                 yamlConfig.Server.Port,
		Host:                 yamlConfig.Server.Host,
		OIDCProviderURL:      yamlConfig.OIDC.ProviderURL,
		OIDCClientID:         yamlConfig.OIDC.ClientID,
		OIDCClientSecret:     yamlConfig.OIDC.ClientSecret,
		OIDCRedirectURL:      yamlConfig.OIDC.RedirectURL,
		UpstreamRoutes:       yamlConfig.Proxy.Routes,
		SessionSecret:        yamlConfig.Session.Secret,
		SessionCookieName:    yamlConfig.Session.CookieName,
		SessionMaxAge:        yamlConfig.Session.MaxAge,
		AllowedOrigins:       yamlConfig.Security.AllowedOrigins,
		TLSCertFile:          yamlConfig.TLS.CertFile,
		TLSKeyFile:           yamlConfig.TLS.KeyFile,
		InsecureSkipVerify:   yamlConfig.TLS.InsecureSkipVerify,
		LogLevel:             yamlConfig.Logging.Level,
		LogFormat:            yamlConfig.Logging.Format,
		HealthEnabled:        yamlConfig.Health.Enabled,
		HealthCheckUpstreams: yamlConfig.Health.CheckUpstreams,
	}

	// Parse OIDC scopes
	if yamlConfig.OIDC.Scopes != "" {
		scopes := strings.Split(yamlConfig.OIDC.Scopes, ",")
		for i, scope := range scopes {
			scopes[i] = strings.TrimSpace(scope)
		}
		config.OIDCScopes = scopes
	}

	// Set defaults for optional fields
	config.setDefaults()

	return config, nil
}

// applyEnvironmentOverrides allows environment variables to override YAML config for sensitive values
func (c *Config) applyEnvironmentOverrides() {
	// Override critical values with environment variables if they exist
	if val := os.Getenv("OIDC_CLIENT_SECRET"); val != "" {
		c.OIDCClientSecret = val
	}
	if val := os.Getenv("SESSION_SECRET"); val != "" {
		c.SessionSecret = val
	}
	if val := os.Getenv("PORT"); val != "" {
		c.Port = val
	}
	if val := os.Getenv("HOST"); val != "" {
		c.Host = val
	}
	if val := os.Getenv("TLS_CERT_FILE"); val != "" {
		c.TLSCertFile = val
	}
	if val := os.Getenv("TLS_KEY_FILE"); val != "" {
		c.TLSKeyFile = val
	}
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		c.LogLevel = val
	}
	if val := os.Getenv("LOG_FORMAT"); val != "" {
		c.LogFormat = val
	}
}

// setDefaults sets default values for optional configuration fields
func (c *Config) setDefaults() {
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
	if c.SessionCookieName == "" {
		c.SessionCookieName = "compas-session"
	}
	if c.SessionMaxAge == 0 {
		c.SessionMaxAge = 3600
	}
	if len(c.OIDCScopes) == 0 {
		c.OIDCScopes = []string{"openid", "profile", "email"}
	}
	if len(c.AllowedOrigins) == 0 {
		c.AllowedOrigins = []string{"*"}
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.LogFormat == "" {
		c.LogFormat = "text"
	}
}

// validate checks if all required configuration values are set
func (c *Config) validate() error {
	required := map[string]string{
		"OIDC Provider URL":  c.OIDCProviderURL,
		"OIDC Client ID":     c.OIDCClientID,
		"OIDC Client Secret": c.OIDCClientSecret,
		"OIDC Redirect URL":  c.OIDCRedirectURL,
		"Session Secret":     c.SessionSecret,
	}

	for key, value := range required {
		if value == "" {
			return fmt.Errorf("required configuration %s is not set", key)
		}
	}

	// Validate upstream routes
	if len(c.UpstreamRoutes) == 0 {
		return fmt.Errorf("no upstream routes configured. Define proxy.routes in YAML configuration")
	}

	// Validate session secret length
	if len(c.SessionSecret) < 32 {
		return fmt.Errorf("session secret must be at least 32 characters long")
	}

	return nil
}

// getEnvOrDefault returns the environment variable value or a default value
