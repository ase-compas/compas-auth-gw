package middleware

import (
	"net/http"
	"net/http/httptest"
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

func TestAddAuthHeadersStripsClientAuthHeaders(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://upstream/api", nil)
	request.Header.Set("X-Auth-User", "client-supplied-user")
	request.Header.Set("X-Auth-Roles", "fleet-mgmt:admin")
	request.Header["x-auth-custom"] = []string{"client-supplied"}

	ctx := SetUserInContext(request.Context(), &UserInfo{
		Sub:   "authenticated-user",
		Email: "jane@example.com",
		Roles: []string{"fleet-mgmt:reader", "fleet-mgmt:writer"},
	})
	ctx = SetAccessTokenInContext(ctx, "access-token")
	request = request.WithContext(ctx)

	middleware := &MultiProxyMiddleware{}
	middleware.addAuthHeaders(request)

	if got := request.Header.Get("X-Auth-User"); got != "authenticated-user" {
		t.Fatalf("X-Auth-User = %q, want the session user", got)
	}
	if got := request.Header.Get("X-Auth-Email"); got != "jane@example.com" {
		t.Fatalf("X-Auth-Email = %q, want the session email", got)
	}
	if got := request.Header.Get("X-Auth-Roles"); got != "fleet-mgmt:reader,fleet-mgmt:writer" {
		t.Fatalf("X-Auth-Roles = %q, want roles from the session", got)
	}
	if got := request.Header.Get("X-Auth-Custom"); got != "" {
		t.Fatalf("client-supplied X-Auth-Custom was forwarded: %q", got)
	}
	if got := request.Header.Get("Authorization"); got != "Bearer access-token" {
		t.Fatalf("Authorization = %q, want the session access token", got)
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
