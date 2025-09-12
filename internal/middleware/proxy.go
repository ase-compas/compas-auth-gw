package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/ase-compas/compas-auth-proxy/internal/config"
)

// ProxyMiddleware handles reverse proxy functionality
type ProxyMiddleware struct {
	config *config.Config
	proxy  *httputil.ReverseProxy
}

// NewProxyMiddleware creates a new proxy middleware
func NewProxyMiddleware(cfg *config.Config) (*ProxyMiddleware, error) {
	upstreamURL, err := url.Parse(cfg.UpstreamURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	// Customize the proxy director to add authentication headers
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Add user information headers if available
		if userInfo := getUserInfoFromContext(req.Context()); userInfo != nil {
			req.Header.Set("X-Auth-User", userInfo.Sub)
			req.Header.Set("X-Auth-Email", userInfo.Email)
			req.Header.Set("X-Auth-Name", userInfo.Name)
			req.Header.Set("X-Auth-Username", userInfo.PreferredUsername)
		}

		// Add access token header if available
		if accessToken := getAccessTokenFromContext(req.Context()); accessToken != "" {
			req.Header.Set("Authorization", "Bearer "+accessToken)
		}

		// Remove hop-by-hop headers
		req.Header.Del("Connection")
		req.Header.Del("Proxy-Connection")
		req.Header.Del("Te")
		req.Header.Del("Trailer")
		req.Header.Del("Upgrade")
	}

	// Customize error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusBadGateway)
	}

	return &ProxyMiddleware{
		config: cfg,
		proxy:  proxy,
	}, nil
}

// Handler returns the proxy handler
func (p *ProxyMiddleware) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers if configured
		p.addCORSHeaders(w, r)

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Proxy the request
		p.proxy.ServeHTTP(w, r)
	})
}

// addCORSHeaders adds CORS headers to the response
func (p *ProxyMiddleware) addCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	// Check if origin is allowed
	if p.isOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else if len(p.config.AllowedOrigins) == 1 && p.config.AllowedOrigins[0] == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// isOriginAllowed checks if the origin is in the allowed list
func (p *ProxyMiddleware) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range p.config.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}

		// Support for wildcard subdomains (e.g., *.example.com)
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*.")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}

	return false
}

// getUserInfoFromContext extracts user info from request context
func getUserInfoFromContext(ctx interface{}) *UserInfo {
	if ctx == nil {
		return nil
	}

	// Type assertion for context.Context
	if c, ok := ctx.(interface {
		Value(key interface{}) interface{}
	}); ok {
		return GetUserFromContext(c.(context.Context))
	}

	return nil
}

// getAccessTokenFromContext extracts access token from request context
func getAccessTokenFromContext(ctx interface{}) string {
	if ctx == nil {
		return ""
	}

	// Type assertion for context.Context
	if c, ok := ctx.(interface {
		Value(key interface{}) interface{}
	}); ok {
		return GetAccessTokenFromContext(c.(context.Context))
	}

	return ""
}
