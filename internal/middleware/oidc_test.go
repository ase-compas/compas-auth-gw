package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ase-compas/compas-auth-proxy/internal/config"
)

func TestHandlerRenewsSessionIdleTimeout(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	middleware := newTestOIDCMiddleware(&config.Config{
		SessionCookieName: "test-session",
		SessionMaxAge:     300,
	}, store, "")

	sessionID := "session-id"
	originalExpiry := time.Now().Add(10 * time.Second)
	if err := store.Set(sessionID, &SessionData{
		UserInfo:             &UserInfo{Sub: "user-id"},
		AccessToken:          "access-token",
		RefreshToken:         "refresh-token",
		ExpiresAt:            originalExpiry,
		AccessTokenExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := GetAccessTokenFromContext(r.Context()); token != "access-token" {
			t.Errorf("expected access token in request context, got %q", token)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	request.AddCookie(&http.Cookie{Name: "test-session", Value: sessionID})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}

	cookie := findCookie(recorder.Result().Cookies(), "test-session")
	if cookie == nil {
		t.Fatal("expected renewed session cookie")
	}
	if cookie.Value != sessionID {
		t.Fatalf("expected session ID %q, got %q", sessionID, cookie.Value)
	}
	if cookie.MaxAge != 300 {
		t.Fatalf("expected cookie max age 300, got %d", cookie.MaxAge)
	}
	if !cookie.Secure {
		t.Fatal("expected renewed cookie to be secure for an HTTPS request")
	}

	storedSession, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("failed to retrieve renewed session: %v", err)
	}
	if !storedSession.ExpiresAt.After(originalExpiry.Add(4 * time.Minute)) {
		t.Fatalf("expected session expiry to be renewed, got %s", storedSession.ExpiresAt)
	}
}

func TestHandlerDoesNotLockSessionDuringDownstreamRequest(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	middleware := newTestOIDCMiddleware(&config.Config{
		SessionCookieName: "test-session",
		SessionMaxAge:     300,
	}, store, "")

	sessionID := "session-id"
	if err := store.Set(sessionID, &SessionData{
		UserInfo:             &UserInfo{Sub: "user-id"},
		AccessToken:          "access-token",
		RefreshToken:         "refresh-token",
		ExpiresAt:            time.Now().Add(5 * time.Minute),
		AccessTokenExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))

	var waitGroup sync.WaitGroup
	for i := 0; i < 2; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			request := httptest.NewRequest(http.MethodGet, "/api", nil)
			request.AddCookie(&http.Cookie{Name: "test-session", Value: sessionID})
			handler.ServeHTTP(httptest.NewRecorder(), request)
		}()
	}

	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			waitGroup.Wait()
			t.Fatal("requests using the same session were serialized downstream")
		}
	}

	close(release)
	waitGroup.Wait()
}

func TestHandlerRefreshesTokenOnceForConcurrentRequests(t *testing.T) {
	var refreshCalls atomic.Int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			http.Error(w, "unexpected grant type", http.StatusBadRequest)
			return
		}

		refreshCalls.Add(1)
		time.Sleep(25 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":300}`)
	}))
	defer tokenServer.Close()

	store := NewMemorySessionStore()
	defer store.Close()

	middleware := newTestOIDCMiddleware(&config.Config{
		OIDCClientID:      "client-id",
		OIDCClientSecret:  "client-secret",
		SessionCookieName: "test-session",
		SessionMaxAge:     300,
	}, store, tokenServer.URL)

	sessionID := "session-id"
	if err := store.Set(sessionID, &SessionData{
		UserInfo:             &UserInfo{Sub: "user-id"},
		AccessToken:          "old-access-token",
		RefreshToken:         "old-refresh-token",
		ExpiresAt:            time.Now().Add(5 * time.Minute),
		AccessTokenExpiresAt: time.Now().Add(10 * time.Second),
	}); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := GetAccessTokenFromContext(r.Context()); token != "new-access-token" {
			http.Error(w, "request used stale access token", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	const requestCount = 8
	start := make(chan struct{})
	errors := make(chan error, requestCount)
	var waitGroup sync.WaitGroup

	for i := 0; i < requestCount; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start

			request := httptest.NewRequest(http.MethodGet, "/api", nil)
			request.AddCookie(&http.Cookie{Name: "test-session", Value: sessionID})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNoContent {
				errors <- fmt.Errorf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
				return
			}
			cookie := findCookie(recorder.Result().Cookies(), "test-session")
			if cookie == nil || cookie.Value != sessionID {
				errors <- fmt.Errorf("expected the existing session ID to be renewed")
			}
		}()
	}

	close(start)
	waitGroup.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
	if calls := refreshCalls.Load(); calls != 1 {
		t.Fatalf("expected one token refresh, got %d", calls)
	}

	storedSession, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("failed to retrieve refreshed session: %v", err)
	}
	if storedSession.AccessToken != "new-access-token" {
		t.Fatalf("expected refreshed access token, got %q", storedSession.AccessToken)
	}
	if storedSession.RefreshToken != "new-refresh-token" {
		t.Fatalf("expected rotated refresh token, got %q", storedSession.RefreshToken)
	}
}

func TestLogoutCannotBeUndoneByConcurrentRefresh(t *testing.T) {
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(refreshStarted)
		<-releaseRefresh
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":300}`)
	}))
	defer tokenServer.Close()

	store := NewMemorySessionStore()
	defer store.Close()

	middleware := newTestOIDCMiddleware(&config.Config{
		OIDCClientID:      "client-id",
		OIDCClientSecret:  "client-secret",
		SessionCookieName: "test-session",
		SessionMaxAge:     300,
	}, store, tokenServer.URL)

	sessionID := "session-id"
	if err := store.Set(sessionID, &SessionData{
		UserInfo:             &UserInfo{Sub: "user-id"},
		AccessToken:          "old-access-token",
		RefreshToken:         "old-refresh-token",
		ExpiresAt:            time.Now().Add(5 * time.Minute),
		AccessTokenExpiresAt: time.Now().Add(10 * time.Second),
	}); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		request := httptest.NewRequest(http.MethodGet, "/api", nil)
		request.AddCookie(&http.Cookie{Name: "test-session", Value: sessionID})
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}()

	<-refreshStarted
	logoutDone := make(chan struct{})
	go func() {
		defer close(logoutDone)
		request := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
		request.AddCookie(&http.Cookie{Name: "test-session", Value: sessionID})
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}()

	select {
	case <-logoutDone:
		// Without session locking logout completes before refresh and the refresh recreates the session.
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseRefresh)
	<-requestDone
	<-logoutDone

	if _, err := store.Get(sessionID); err == nil {
		t.Fatal("expected logout to leave the session deleted")
	}
}

func TestTokenTimingUsesProportionalSkewForShortLivedTokens(t *testing.T) {
	issuedAt := time.Now()
	expiresAt, refreshAt, err := tokenTiming(issuedAt, &TokenResponse{
		AccessToken: "access-token",
		ExpiresIn:   10,
	})
	if err != nil {
		t.Fatalf("unexpected token timing error: %v", err)
	}

	if !expiresAt.Equal(issuedAt.Add(10 * time.Second)) {
		t.Fatalf("unexpected token expiry: %s", expiresAt)
	}
	if !refreshAt.Equal(issuedAt.Add(9 * time.Second)) {
		t.Fatalf("unexpected refresh time: %s", refreshAt)
	}
}

func TestMemorySessionStoreReturnsCopies(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	sessionID := "session-id"
	if err := store.Set(sessionID, &SessionData{
		UserInfo:    &UserInfo{Sub: "user-id"},
		AccessToken: "access-token",
		ExpiresAt:   time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	first, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("failed to retrieve session: %v", err)
	}
	first.AccessToken = "mutated-token"
	first.UserInfo.Sub = "mutated-user"

	second, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("failed to retrieve session again: %v", err)
	}
	if second.AccessToken != "access-token" {
		t.Fatalf("stored access token was mutated: %q", second.AccessToken)
	}
	if second.UserInfo.Sub != "user-id" {
		t.Fatalf("stored user info was mutated: %q", second.UserInfo.Sub)
	}
}

func newTestOIDCMiddleware(cfg *config.Config, store SessionStore, tokenEndpoint string) *OIDCMiddleware {
	return &OIDCMiddleware{
		config:       cfg,
		httpClient:   http.DefaultClient,
		sessionStore: store,
		providerConfig: &ProviderConfig{
			AuthorizationEndpoint: "https://identity.example/authorize",
			TokenEndpoint:         tokenEndpoint,
		},
		stateRedirect: make(map[string]string),
		sessionLocks:  make(map[string]*sessionRequestLock),
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
