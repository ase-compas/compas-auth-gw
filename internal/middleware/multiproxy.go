package middleware

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/ase-compas/compas-auth-proxy/internal/config"
)

// MultiProxyMiddleware handles reverse proxy functionality with multiple upstreams
type MultiProxyMiddleware struct {
	config *config.Config
	routes []ProxyRoute
}

// ProxyRoute represents a configured proxy route
type ProxyRoute struct {
	PathPrefix  string
	UpstreamURL *url.URL
	Proxy       *httputil.ReverseProxy
	StripPath   bool
}

// NewMultiProxyMiddleware creates a new multi-upstream proxy middleware
func NewMultiProxyMiddleware(cfg *config.Config) (*MultiProxyMiddleware, error) {
	middleware := &MultiProxyMiddleware{
		config: cfg,
		routes: make([]ProxyRoute, 0, len(cfg.UpstreamRoutes)),
	}

	// Create proxy routes from configuration
	for _, routeConfig := range cfg.UpstreamRoutes {
		upstreamURL, err := url.Parse(routeConfig.UpstreamURL)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream URL %s: %v", routeConfig.UpstreamURL, err)
		}

		proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

		// Customize the proxy director
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			// Call original director first
			originalDirector(req)

			// Add authentication headers
			middleware.addAuthHeaders(req)

			// Strip path prefix if configured
			if routeConfig.StripPath && routeConfig.Path != "/" {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, strings.TrimSuffix(routeConfig.Path, "/"))
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
			}

			// Remove hop-by-hop headers
			req.Header.Del("Connection")
			req.Header.Del("Proxy-Connection")
			req.Header.Del("Te")
			req.Header.Del("Trailer")
			req.Header.Del("Upgrade")

			log.Printf("Proxying %s %s to %s%s", req.Method, req.URL.Path, upstreamURL.String(), req.URL.Path)
		}

		// Customize error handler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error for %s: %v", routeConfig.UpstreamURL, err)
			http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusBadGateway)
		}

		route := ProxyRoute{
			PathPrefix:  routeConfig.Path,
			UpstreamURL: upstreamURL,
			Proxy:       proxy,
			StripPath:   routeConfig.StripPath,
		}

		middleware.routes = append(middleware.routes, route)
	}

	// Sort routes by path length (longest first) for proper matching
	middleware.sortRoutes()

	log.Printf("Configured %d proxy routes:", len(middleware.routes))
	for _, route := range middleware.routes {
		log.Printf("  %s -> %s (strip: %v)", route.PathPrefix, route.UpstreamURL.String(), route.StripPath)
	}

	return middleware, nil
}

// Handler returns the proxy handler
func (m *MultiProxyMiddleware) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers if configured
		m.addCORSHeaders(w, r)

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Find matching route
		route := m.findRoute(r.URL.Path)
		if route == nil {
			log.Printf("No route found for path: %s", r.URL.Path)
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		// Proxy the request
		route.Proxy.ServeHTTP(w, r)
	})
}

// findRoute finds the best matching route for a given path
func (m *MultiProxyMiddleware) findRoute(path string) *ProxyRoute {
	for _, route := range m.routes {
		if m.pathMatches(path, route.PathPrefix) {
			return &route
		}
	}
	return nil
}

// pathMatches checks if a path matches a route prefix
func (m *MultiProxyMiddleware) pathMatches(path, prefix string) bool {
	if prefix == "/" {
		return true // Root path matches everything
	}

	// Ensure prefix ends with / for proper matching
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// Ensure path ends with / for comparison
	pathForComparison := path
	if !strings.HasSuffix(pathForComparison, "/") {
		pathForComparison += "/"
	}

	return strings.HasPrefix(pathForComparison, prefix) || path == strings.TrimSuffix(prefix, "/")
}

// sortRoutes sorts routes by path length (longest first) for proper matching
func (m *MultiProxyMiddleware) sortRoutes() {
	// Simple bubble sort by path length (longest first)
	n := len(m.routes)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if len(m.routes[j].PathPrefix) < len(m.routes[j+1].PathPrefix) {
				m.routes[j], m.routes[j+1] = m.routes[j+1], m.routes[j]
			}
		}
	}
}

// addAuthHeaders adds authentication headers to the request
func (m *MultiProxyMiddleware) addAuthHeaders(req *http.Request) {
	// Add user information headers if available
	if userInfo := GetUserFromContext(req.Context()); userInfo != nil {
		req.Header.Set("X-Auth-User", userInfo.Sub)
		req.Header.Set("X-Auth-Email", userInfo.Email)
		req.Header.Set("X-Auth-Name", userInfo.Name)
		req.Header.Set("X-Auth-Username", userInfo.PreferredUsername)
	}

	// Add access token header if available
	if accessToken := GetAccessTokenFromContext(req.Context()); accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
}

// addCORSHeaders adds CORS headers to the response
func (m *MultiProxyMiddleware) addCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	// Check if origin is allowed
	if m.isOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else if len(m.config.AllowedOrigins) == 1 && m.config.AllowedOrigins[0] == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// isOriginAllowed checks if the origin is in the allowed list
func (m *MultiProxyMiddleware) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range m.config.AllowedOrigins {
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
