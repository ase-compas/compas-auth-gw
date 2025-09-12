package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ase-compas/compas-auth-proxy/internal/config"
	"github.com/ase-compas/compas-auth-proxy/internal/middleware"
)

func main() {

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create session store
	sessionStore := middleware.NewMemorySessionStore()
	defer sessionStore.Close()

	// Create OIDC middleware
	oidcMiddleware, err := middleware.NewOIDCMiddleware(cfg, sessionStore)
	if err != nil {
		log.Fatalf("Failed to create OIDC middleware: %v", err)
	}

	// Create proxy middleware
	proxyMiddleware, err := middleware.NewProxyMiddleware(cfg)
	if err != nil {
		log.Fatalf("Failed to create proxy middleware: %v", err)
	}

	// Create router
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339))
	})

	// OIDC callback endpoint
	mux.HandleFunc("/auth/callback", oidcMiddleware.HandleCallback)

	// User info endpoint
	mux.HandleFunc("/auth/userinfo", func(w http.ResponseWriter, r *http.Request) {
		// This endpoint requires authentication
		userInfo := getUserInfoFromRequest(r)
		if userInfo == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"sub": "%s",
			"name": "%s",
			"email": "%s",
			"preferred_username": "%s"
		}`, userInfo.Sub, userInfo.Name, userInfo.Email, userInfo.PreferredUsername)
	})

	// All other requests go through the proxy
	mux.Handle("/", oidcMiddleware.Handler(proxyMiddleware.Handler()))

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	go func() {
		log.Printf("Starting CoMPAS Auth Proxy on %s:%s", cfg.Host, cfg.Port)
		log.Printf("OIDC Provider: %s", cfg.OIDCProviderURL)
		log.Printf("Upstream: %s", cfg.UpstreamURL)

		var err error
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			log.Println("Starting HTTPS server")
			err = server.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			log.Println("Starting HTTP server")
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	server.Close()
	log.Println("Server stopped")
}

// loggingMiddleware provides request logging
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapper := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)
		log.Printf("%s %s %d %v %s", r.Method, r.URL.Path, wrapper.statusCode, duration, r.RemoteAddr)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// getUserInfoFromRequest extracts user info from request context
func getUserInfoFromRequest(r *http.Request) *middleware.UserInfo {
	if user := r.Context().Value("user"); user != nil {
		if userInfo, ok := user.(*middleware.UserInfo); ok {
			return userInfo
		}
	}
	return nil
}
