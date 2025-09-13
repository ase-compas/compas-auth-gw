package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadFromYAML(t *testing.T) {
	// Create a temporary YAML config file
	yamlContent := `
server:
  port: "9090"
  host: "127.0.0.1"

oidc:
  provider_url: "http://test-provider.com"
  client_id: "test-client"
  client_secret: "test-secret-very-long-to-meet-requirements"
  redirect_url: "http://localhost:9090/auth/callback"
  scopes: "openid,profile,email"

session:
  secret: "test-session-secret-very-long-to-meet-32-char-requirement"
  cookie_name: "test-session"
  max_age: 7200

proxy:
  routes:
    - path: "/api/test"
      upstream_url: "http://test-backend:8080"
      strip_path: true
    - path: "/"
      upstream_url: "http://test-frontend:3000"
      strip_path: false

security:
  allowed_origins:
    - "http://localhost:3000"
    - "http://localhost:9090"

logging:
  level: "debug"
  format: "json"

health:
  enabled: true
  check_upstreams: false
`

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Load configuration from YAML
	config, err := LoadFromYAML(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load YAML config: %v", err)
	}

	// Verify configuration values
	if config.Port != "9090" {
		t.Errorf("Expected port 9090, got %s", config.Port)
	}

	if config.Host != "127.0.0.1" {
		t.Errorf("Expected host 127.0.0.1, got %s", config.Host)
	}

	if config.OIDCProviderURL != "http://test-provider.com" {
		t.Errorf("Expected OIDC provider URL http://test-provider.com, got %s", config.OIDCProviderURL)
	}

	if len(config.UpstreamRoutes) != 2 {
		t.Errorf("Expected 2 upstream routes, got %d", len(config.UpstreamRoutes))
	}

	if config.UpstreamRoutes[0].Path != "/api/test" {
		t.Errorf("Expected first route path /api/test, got %s", config.UpstreamRoutes[0].Path)
	}

	if !config.UpstreamRoutes[0].StripPath {
		t.Errorf("Expected first route strip_path to be true")
	}

	if config.LogLevel != "debug" {
		t.Errorf("Expected log level debug, got %s", config.LogLevel)
	}

	if config.LogFormat != "json" {
		t.Errorf("Expected log format json, got %s", config.LogFormat)
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// Create a temporary YAML config file
	yamlContent := `
server:
  port: "8080"
  host: "0.0.0.0"

oidc:
  provider_url: "http://test-provider.com"
  client_id: "test-client"
  client_secret: "original-secret-very-long-to-meet-requirements"
  redirect_url: "http://localhost:8080/auth/callback"
  scopes: "openid,profile,email"

session:
  secret: "original-session-secret-very-long-to-meet-32-char-requirement"
  cookie_name: "test-session"
  max_age: 3600

proxy:
  routes:
    - path: "/"
      upstream_url: "http://test-backend:8080"
      strip_path: false

security:
  allowed_origins:
    - "http://localhost:3000"
`

	tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Set environment variables to override YAML values
	os.Setenv("OIDC_CLIENT_SECRET", "env-override-secret-very-long-to-meet-requirements")
	os.Setenv("SESSION_SECRET", "env-override-session-secret-very-long-to-meet-32-char-requirement")
	os.Setenv("PORT", "9999")
	defer func() {
		os.Unsetenv("OIDC_CLIENT_SECRET")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("PORT")
	}()

	// Load configuration from YAML
	config, err := LoadFromYAML(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load YAML config: %v", err)
	}

	// Apply environment overrides
	config.applyEnvironmentOverrides()

	// Verify that environment variables override YAML values for sensitive data
	if config.OIDCClientSecret != "env-override-secret-very-long-to-meet-requirements" {
		t.Errorf("Expected OIDC client secret to be overridden by env var, got %s", config.OIDCClientSecret)
	}

	if config.SessionSecret != "env-override-session-secret-very-long-to-meet-32-char-requirement" {
		t.Errorf("Expected session secret to be overridden by env var, got %s", config.SessionSecret)
	}

	if config.Port != "9999" {
		t.Errorf("Expected port to be overridden by env var, got %s", config.Port)
	}
}

func TestRequireConfigFile(t *testing.T) {
	// Clear CONFIG_FILE environment variable
	originalConfigFile := os.Getenv("CONFIG_FILE")
	os.Unsetenv("CONFIG_FILE")
	defer func() {
		if originalConfigFile != "" {
			os.Setenv("CONFIG_FILE", originalConfigFile)
		}
	}()

	// LoadConfig should fail without CONFIG_FILE
	_, err := LoadConfig()
	if err == nil {
		t.Error("Expected LoadConfig to fail when CONFIG_FILE is not set")
	}

	expectedErrorSubstring := "CONFIG_FILE environment variable must be set"
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error to contain '%s', got: %v", expectedErrorSubstring, err)
	}
}

func TestConfigValidation(t *testing.T) {
	// Test with invalid config (missing required fields)
	config := &Config{
		Port: "8080",
		Host: "0.0.0.0",
		// Missing required OIDC fields
	}

	err := config.validate()
	if err == nil {
		t.Error("Expected validation to fail for missing required fields")
	}

	// Test with valid config
	config = &Config{
		Port:             "8080",
		Host:             "0.0.0.0",
		OIDCProviderURL:  "http://provider.com",
		OIDCClientID:     "client-id",
		OIDCClientSecret: "client-secret",
		OIDCRedirectURL:  "http://localhost:8080/callback",
		SessionSecret:    "very-long-session-secret-that-meets-minimum-requirements",
		UpstreamRoutes:   []UpstreamRoute{{Path: "/", UpstreamURL: "http://backend", StripPath: false}},
	}

	err = config.validate()
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}

	// Test with short session secret
	config.SessionSecret = "short"
	err = config.validate()
	if err == nil {
		t.Error("Expected validation to fail for short session secret")
	}
}
