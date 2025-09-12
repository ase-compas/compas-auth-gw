package middleware

import (
	"testing"

	"github.com/ase-compas/compas-auth-proxy/internal/config"
)

func TestMultiProxyRouting(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		UpstreamRoutes: []config.UpstreamRoute{
			{Path: "/api/scl", UpstreamURL: "http://scl-service:8081", StripPath: true},
			{Path: "/api/history", UpstreamURL: "http://history-service:8082", StripPath: true},
			{Path: "/api", UpstreamURL: "http://api-service:8083", StripPath: false},
			{Path: "/", UpstreamURL: "http://frontend:80", StripPath: false},
		},
	}

	// Create multi-proxy middleware
	middleware, err := NewMultiProxyMiddleware(cfg)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Test cases
	testCases := []struct {
		path     string
		expected string
	}{
		{"/api/scl/files", "/api/scl"},
		{"/api/scl", "/api/scl"},
		{"/api/history/recent", "/api/history"},
		{"/api/other", "/api"},
		{"/health", "/"},
		{"/login", "/"},
		{"", "/"},
	}

	for _, tc := range testCases {
		route := middleware.findRoute(tc.path)
		if route == nil {
			t.Errorf("No route found for path: %s", tc.path)
			continue
		}

		if route.PathPrefix != tc.expected {
			t.Errorf("Path %s: expected route %s, got %s", tc.path, tc.expected, route.PathPrefix)
		}
	}
}

func TestPathMatching(t *testing.T) {
	middleware := &MultiProxyMiddleware{}

	testCases := []struct {
		path     string
		prefix   string
		expected bool
	}{
		{"/api/scl/files", "/api/scl", true},
		{"/api/scl", "/api/scl", true},
		{"/api/scl/", "/api/scl", true},
		{"/api", "/api/scl", false},
		{"/api/other", "/api/scl", false},
		{"/health", "/", true},
		{"", "/", true},
		{"/api/scl/deep/path", "/api/scl", true},
	}

	for _, tc := range testCases {
		result := middleware.pathMatches(tc.path, tc.prefix)
		if result != tc.expected {
			t.Errorf("pathMatches(%s, %s): expected %v, got %v", tc.path, tc.prefix, tc.expected, result)
		}
	}
}
