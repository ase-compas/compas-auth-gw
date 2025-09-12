package config

import (
	"fmt"
	"os"
	"strconv"
)

// UpstreamRoute represents a routing rule for upstream services
type UpstreamRoute struct {
	Path        string `json:"path"`        // URL path prefix to match
	UpstreamURL string `json:"upstream_url"` // Target upstream URL
	StripPath   bool   `json:"strip_path"`   // Whether to strip the path prefix when forwarding
}

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
	UpstreamURL    string          // Legacy single upstream (deprecated)
	UpstreamRoutes []UpstreamRoute // New multi-upstream configuration
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

	// Parse upstream routes
	if err := config.parseUpstreamRoutes(); err != nil {
		return nil, fmt.Errorf("failed to parse upstream routes: %v", err)
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
		"SESSION_SECRET":     c.SessionSecret,
	}

	for key, value := range required {
		if value == "" {
			return fmt.Errorf("required environment variable %s is not set", key)
		}
	}
	
	// Validate upstream routes
	if len(c.UpstreamRoutes) == 0 {
		return fmt.Errorf("no upstream routes configured. Set UPSTREAM_ROUTES or UPSTREAM_URL")
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

// parseUpstreamRoutes parses upstream route configuration from environment variables
func (c *Config) parseUpstreamRoutes() error {
	// Parse upstream routes from environment variables
	// Format: UPSTREAM_ROUTES="path1:url1:strip,path2:url2:strip,..."
	routesEnv := getEnvOrDefault("UPSTREAM_ROUTES", "")
	
	if routesEnv != "" {
		routes := parseStringSlice(routesEnv)
		for _, route := range routes {
			parts := splitString(route, ":")
			if len(parts) < 2 || len(parts) > 3 {
				return fmt.Errorf("invalid route format: %s (expected path:url or path:url:strip)", route)
			}
			
			path := trimSpace(parts[0])
			upstreamURL := trimSpace(parts[1])
			stripPath := false
			
			if len(parts) == 3 {
				stripPath = trimSpace(parts[2]) == "true"
			}
			
			if path == "" || upstreamURL == "" {
				return fmt.Errorf("invalid route: path and URL cannot be empty")
			}
			
			c.UpstreamRoutes = append(c.UpstreamRoutes, UpstreamRoute{
				Path:        path,
				UpstreamURL: upstreamURL,
				StripPath:   stripPath,
			})
		}
	}
	
	// If no routes configured, create a default route from legacy UPSTREAM_URL
	if len(c.UpstreamRoutes) == 0 && c.UpstreamURL != "" {
		c.UpstreamRoutes = append(c.UpstreamRoutes, UpstreamRoute{
			Path:        "/",
			UpstreamURL: c.UpstreamURL,
			StripPath:   false,
		})
	}
	
	return nil
}
