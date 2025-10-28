package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ase-compas/compas-auth-proxy/internal/config"
)

// OIDCMiddleware handles OpenID Connect authentication
type OIDCMiddleware struct {
	config         *config.Config
	httpClient     *http.Client
	providerConfig *ProviderConfig
	sessionStore   SessionStore
}

// ProviderConfig represents OpenID Connect provider configuration
type ProviderConfig struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSEndpoint          string `json:"jwks_uri"`
}

// TokenResponse represents the OAuth2 token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
}

// UserInfo represents user information from the provider
type UserInfo struct {
	Sub               string `json:"sub"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
}

// SessionStore interface for session management
type SessionStore interface {
	Get(sessionID string) (*SessionData, error)
	Set(sessionID string, data *SessionData) error
	Delete(sessionID string) error
}

// SessionData represents session information
type SessionData struct {
	UserInfo    *UserInfo `json:"user_info"`
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
	State       string    `json:"state"`
}

// NewOIDCMiddleware creates a new OIDC middleware instance
func NewOIDCMiddleware(cfg *config.Config, sessionStore SessionStore) (*OIDCMiddleware, error) {
	middleware := &OIDCMiddleware{
		config:       cfg,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		sessionStore: sessionStore,
	}

	// Discover provider configuration
	if err := middleware.discoverProvider(); err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider: %v", err)
	}

	return middleware, nil
}

// discoverProvider discovers the OIDC provider configuration
func (m *OIDCMiddleware) discoverProvider() error {
	discoveryURL := strings.TrimSuffix(m.config.OIDCProviderURL, "/") + "/.well-known/openid-configuration"

	resp, err := m.httpClient.Get(discoveryURL)
	if err != nil {
		return fmt.Errorf("failed to fetch provider config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("provider config request failed with status: %d", resp.StatusCode)
	}

	var providerConfig ProviderConfig
	if err := json.NewDecoder(resp.Body).Decode(&providerConfig); err != nil {
		return fmt.Errorf("failed to decode provider config: %v", err)
	}

	m.providerConfig = &providerConfig
	return nil
}

// Handler returns the middleware handler function
func (m *OIDCMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Skip authentication for health check and callback endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/oidc/callback" {
			log.Printf("Skipping authentication for: %s", r.URL.Path)
			next.ServeHTTP(w, r)
			return
		}

		// Handle logout
		if r.URL.Path == "/auth/logout" {
			log.Printf("Handling logout request")
			m.handleLogout(w, r)
			return
		}

		// Check if user is authenticated
		sessionID := m.getSessionID(r)
		if sessionID == "" {
			log.Printf("No session ID found, redirecting to login")
			m.redirectToLogin(w, r)
			return
		}

		sessionData, err := m.sessionStore.Get(sessionID)
		if err != nil || sessionData == nil || sessionData.ExpiresAt.Before(time.Now()) {
			log.Printf("Invalid or expired session %s, redirecting to login", sessionID)
			m.redirectToLogin(w, r)
			return
		}

		// Add user information to request context
		ctx := SetUserInContext(r.Context(), sessionData.UserInfo)
		ctx = SetAccessTokenInContext(ctx, sessionData.AccessToken)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HandleCallback handles the OIDC callback
func (m *OIDCMiddleware) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state parameter
	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "Missing state parameter", http.StatusBadRequest)
		return
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens
	tokenResp, err := m.exchangeCodeForToken(code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Token exchange failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Get user info
	userInfo, err := m.getUserInfo(tokenResp.AccessToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get user info: %v", err), http.StatusInternalServerError)
		return
	}

	// Create session
	sessionID := m.generateSessionID()
	sessionData := &SessionData{
		UserInfo:    userInfo,
		AccessToken: tokenResp.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(m.config.SessionMaxAge) * time.Second),
		State:       state,
	}

	if err := m.sessionStore.Set(sessionID, sessionData); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	m.setSessionCookie(w, sessionID)

	// Redirect to original URL or home
	redirectURL := "/"
	if originalURL := r.URL.Query().Get("redirect_uri"); originalURL != "" {
		redirectURL = originalURL
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// redirectToLogin redirects the user to the OIDC provider for authentication
func (m *OIDCMiddleware) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	state := m.generateState()

	authURL, _ := url.Parse(m.providerConfig.AuthorizationEndpoint)
	query := authURL.Query()
	query.Set("client_id", m.config.OIDCClientID)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(m.config.OIDCScopes, " "))
	query.Set("redirect_uri", m.config.OIDCRedirectURL)
	query.Set("state", state)
	authURL.RawQuery = query.Encode()

	http.Redirect(w, r, authURL.String(), http.StatusFound)
}

// exchangeCodeForToken exchanges authorization code for access token
func (m *OIDCMiddleware) exchangeCodeForToken(code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", m.config.OIDCRedirectURL)
	data.Set("client_id", m.config.OIDCClientID)
	data.Set("client_secret", m.config.OIDCClientSecret)

	req, err := http.NewRequest("POST", m.providerConfig.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// getUserInfo retrieves user information using the access token
func (m *OIDCMiddleware) getUserInfo(accessToken string) (*UserInfo, error) {
	req, err := http.NewRequest("GET", m.providerConfig.UserInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status: %d", resp.StatusCode)
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// handleLogout handles user logout
func (m *OIDCMiddleware) handleLogout(w http.ResponseWriter, r *http.Request) {
	sessionID := m.getSessionID(r)
	if sessionID != "" {
		m.sessionStore.Delete(sessionID)
		log.Printf("User logged out, session %s deleted", sessionID)
	}

	// Clear session cookie
	cookie := &http.Cookie{
		Name:     m.config.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
	}
	http.SetCookie(w, cookie)
	log.Printf("Session cookie %s cleared", m.config.SessionCookieName)

	http.Redirect(w, r, "/", http.StatusFound)
}

// getSessionID extracts session ID from request cookie
func (m *OIDCMiddleware) getSessionID(r *http.Request) string {
	cookie, err := r.Cookie(m.config.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// setSessionCookie sets the session cookie
func (m *OIDCMiddleware) setSessionCookie(w http.ResponseWriter, sessionID string) {
	cookie := &http.Cookie{
		Name:     m.config.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   m.config.SessionMaxAge,
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

// generateState generates a random state parameter
func (m *OIDCMiddleware) generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// generateSessionID generates a random session ID
func (m *OIDCMiddleware) generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
