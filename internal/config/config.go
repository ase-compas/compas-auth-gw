package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the application configuration
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
	UpstreamURL       string
	SessionSecret     string
	SessionCookieName string
	SessionMaxAge     int

	// Security configuration
	AllowedOrigins     []string
	TLSCertFile        string
	TLSKeyFile         string
	InsecureSkipVerify bool
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		Port:              getEnvOrDefault("PORT", "8080"),
		Host:              getEnvOrDefault("HOST", "0.0.0.0"),
		OIDCProviderURL:   getEnvOrDefault("OIDC_PROVIDER_URL", ""),
		OIDCClientID:      getEnvOrDefault("OIDC_CLIENT_ID", ""),
		OIDCClientSecret:  getEnvOrDefault("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:   getEnvOrDefault("OIDC_REDIRECT_URL", ""),
		UpstreamURL:       getEnvOrDefault("UPSTREAM_URL", ""),
		SessionSecret:     getEnvOrDefault("SESSION_SECRET", ""),
		SessionCookieName: getEnvOrDefault("SESSION_COOKIE_NAME", "compas-session"),
		TLSCertFile:       getEnvOrDefault("TLS_CERT_FILE", ""),
		TLSKeyFile:        getEnvOrDefault("TLS_KEY_FILE", ""),
	}

	// Parse session max age
	sessionMaxAge := getEnvOrDefault("SESSION_MAX_AGE", "3600")
	maxAge, err := strconv.Atoi(sessionMaxAge)
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_MAX_AGE: %v", err)
	}
	config.SessionMaxAge = maxAge

	// Parse insecure skip verify
	insecureSkipVerify := getEnvOrDefault("INSECURE_SKIP_VERIFY", "false")
	config.InsecureSkipVerify = insecureSkipVerify == "true"

	// Parse scopes (comma-separated)
	scopesStr := getEnvOrDefault("OIDC_SCOPES", "openid,profile,email")
	config.OIDCScopes = parseStringSlice(scopesStr)

	// Parse allowed origins (comma-separated)
	originsStr := getEnvOrDefault("ALLOWED_ORIGINS", "*")
	config.AllowedOrigins = parseStringSlice(originsStr)

	// Validate required fields
	if err := config.validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// validate checks if all required configuration values are set
func (c *Config) validate() error {
	required := map[string]string{
		"OIDC_PROVIDER_URL":  c.OIDCProviderURL,
		"OIDC_CLIENT_ID":     c.OIDCClientID,
		"OIDC_CLIENT_SECRET": c.OIDCClientSecret,
		"OIDC_REDIRECT_URL":  c.OIDCRedirectURL,
		"UPSTREAM_URL":       c.UpstreamURL,
		"SESSION_SECRET":     c.SessionSecret,
	}

	for key, value := range required {
		if value == "" {
			return fmt.Errorf("required environment variable %s is not set", key)
		}
	}

	return nil
}

// getEnvOrDefault returns the environment variable value or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseStringSlice parses a comma-separated string into a slice
func parseStringSlice(s string) []string {
	if s == "" {
		return []string{}
	}

	var result []string
	for _, item := range splitAndTrim(s, ",") {
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// splitAndTrim splits a string and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	var result []string
	for _, item := range splitString(s, sep) {
		trimmed := trimSpace(item)
		result = append(result, trimmed)
	}
	return result
}

// splitString splits a string by separator
func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}

	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)

	// Find first non-space character
	for start < end && isSpace(s[start]) {
		start++
	}

	// Find last non-space character
	for end > start && isSpace(s[end-1]) {
		end--
	}

	return s[start:end]
}

// isSpace checks if a character is whitespace
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
