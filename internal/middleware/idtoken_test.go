package middleware

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ase-compas/compas-auth-proxy/internal/config"
)

const (
	testIssuer   = "https://issuer.example"
	testClientID = "test-client"
)

func testJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		panic(err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func testIDTokenClaims() map[string]any {
	return map[string]any{
		"iss":                testIssuer,
		"aud":                testClientID,
		"sub":                "id-token-sub",
		"exp":                time.Now().Add(time.Hour).Unix(),
		"name":               "ID Token User",
		"email":              "id-token@example.com",
		"preferred_username": "id-token-user",
		"oid":                "id-token-oid",
		"tid":                "id-token-tid",
	}
}

func newUserInfoTestMiddleware(source string) (*OIDCMiddleware, *MemorySessionStore) {
	store := NewMemorySessionStore()
	middleware := newTestOIDCMiddleware(&config.Config{
		OIDCClientID:       testClientID,
		OIDCUserInfoSource: source,
		SessionCookieName:  "test-session",
		SessionMaxAge:      300,
	}, store, "")
	middleware.providerConfig.Issuer = testIssuer
	return middleware, store
}

func TestParseJWTClaimsSupportsStringAndArrayAudience(t *testing.T) {
	for name, audience := range map[string]any{
		"string": testClientID,
		"array":  []string{testClientID, "other-client"},
	} {
		t.Run(name, func(t *testing.T) {
			claims, err := parseJWTClaims(testJWT(map[string]any{
				"iss": testIssuer,
				"aud": audience,
				"sub": "user",
				"exp": time.Now().Add(time.Hour).Unix(),
			}))
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			if !claims.Audience.contains(testClientID) {
				t.Fatalf("expected audience to contain %q, got %v", testClientID, claims.Audience)
			}
		})
	}
}

func TestUserInfoFromIDToken(t *testing.T) {
	middleware, store := newUserInfoTestMiddleware("id_token")
	defer store.Close()

	userInfo, err := middleware.userInfoFromIDToken(testJWT(testIDTokenClaims()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userInfo.Sub != "id-token-sub" || userInfo.Name != "ID Token User" ||
		userInfo.Email != "id-token@example.com" || userInfo.PreferredUsername != "id-token-user" ||
		userInfo.OID != "id-token-oid" || userInfo.TID != "id-token-tid" {
		t.Fatalf("unexpected user info: %+v", userInfo)
	}
}

func TestUserInfoFromIDTokenRejectsInvalidClaims(t *testing.T) {
	middleware, store := newUserInfoTestMiddleware("id_token")
	defer store.Close()

	tests := map[string]func(map[string]any){
		"expired": func(claims map[string]any) {
			claims["exp"] = time.Now().Add(-time.Minute).Unix()
		},
		"wrong issuer": func(claims map[string]any) {
			claims["iss"] = "https://other-issuer.example"
		},
		"wrong audience": func(claims map[string]any) {
			claims["aud"] = "other-client"
		},
		"missing sub": func(claims map[string]any) {
			delete(claims, "sub")
		},
		"missing exp": func(claims map[string]any) {
			delete(claims, "exp")
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			claims := testIDTokenClaims()
			mutate(claims)
			if _, err := middleware.userInfoFromIDToken(testJWT(claims)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestResolveUserInfoUsesIDTokenWithoutUserinfo(t *testing.T) {
	middleware, store := newUserInfoTestMiddleware("id_token")
	defer store.Close()

	// If the ID token is usable, the userinfo endpoint must not be called.
	middleware.providerConfig.UserInfoEndpoint = "http://127.0.0.1:1/userinfo"

	userInfo := middleware.resolveUserInfo(&TokenResponse{
		AccessToken: "not-a-jwt",
		IDToken:     testJWT(testIDTokenClaims()),
	})
	if userInfo.Sub != "id-token-sub" {
		t.Fatalf("expected ID-token user, got %+v", userInfo)
	}
}

func TestResolveUserInfoIDTokenAcceptsMissingOptionalClaims(t *testing.T) {
	middleware, store := newUserInfoTestMiddleware("id_token")
	defer store.Close()

	// A valid sub is enough in id_token mode; missing name/email/preferred_username
	// must not cause a fallback or an error.
	middleware.providerConfig.UserInfoEndpoint = "http://127.0.0.1:1/userinfo"

	userInfo := middleware.resolveUserInfo(&TokenResponse{
		AccessToken: "not-a-jwt",
		IDToken: testJWT(map[string]any{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "id-token-sub",
			"exp": time.Now().Add(time.Hour).Unix(),
		}),
	})
	if userInfo.Sub != "id-token-sub" {
		t.Fatalf("expected ID-token subject, got %+v", userInfo)
	}
	if userInfo.Name != "" || userInfo.Email != "" || userInfo.PreferredUsername != "" {
		t.Fatalf("expected optional claims to remain empty, got %+v", userInfo)
	}
}

func TestResolveUserInfoAutoPrefersCompleteIDToken(t *testing.T) {
	middleware, store := newUserInfoTestMiddleware("auto")
	defer store.Close()

	middleware.providerConfig.UserInfoEndpoint = "http://127.0.0.1:1/userinfo"

	userInfo := middleware.resolveUserInfo(&TokenResponse{
		AccessToken: "not-a-jwt",
		IDToken:     testJWT(testIDTokenClaims()),
	})
	if userInfo.Sub != "id-token-sub" || userInfo.Name != "ID Token User" {
		t.Fatalf("expected ID-token user, got %+v", userInfo)
	}
}

func TestResolveUserInfoFallsBackToUserinfo(t *testing.T) {
	middleware, store := newUserInfoTestMiddleware("id_token")
	defer store.Close()

	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Errorf("unexpected Authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"sub":"userinfo-sub","name":"User Info","email":"userinfo@example.com","preferred_username":"userinfo-user"}`)
	}))
	defer userinfoServer.Close()
	middleware.providerConfig.UserInfoEndpoint = userinfoServer.URL

	userInfo := middleware.resolveUserInfo(&TokenResponse{
		AccessToken: "access-token",
		IDToken:     "not-a-jwt",
	})
	if userInfo.Sub != "userinfo-sub" || userInfo.Name != "User Info" {
		t.Fatalf("expected userinfo fallback, got %+v", userInfo)
	}
}

func TestResolveUserInfoAutoFallsBackWhenIDTokenHasNoProfileClaims(t *testing.T) {
	middleware, store := newUserInfoTestMiddleware("auto")
	defer store.Close()

	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"sub":"userinfo-sub","name":"User Info"}`)
	}))
	defer userinfoServer.Close()
	middleware.providerConfig.UserInfoEndpoint = userinfoServer.URL

	userInfo := middleware.resolveUserInfo(&TokenResponse{
		AccessToken: "access-token",
		IDToken: testJWT(map[string]any{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "id-token-sub",
			"exp": time.Now().Add(time.Hour).Unix(),
		}),
	})
	if userInfo.Sub != "userinfo-sub" {
		t.Fatalf("expected userinfo fallback, got %+v", userInfo)
	}
}

func TestResolveUserInfoFallsBackToAccessToken(t *testing.T) {
	middleware, store := newUserInfoTestMiddleware("id_token")
	defer store.Close()

	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer userinfoServer.Close()
	middleware.providerConfig.UserInfoEndpoint = userinfoServer.URL

	accessToken := testJWT(map[string]any{
		"sub":                "access-token-sub",
		"name":               "Access Token User",
		"email":              "access-token@example.com",
		"preferred_username": "access-token-user",
		"oid":                "access-token-oid",
	})

	userInfo := middleware.resolveUserInfo(&TokenResponse{
		AccessToken: accessToken,
		IDToken:     "not-a-jwt",
	})
	if userInfo.Sub != "access-token-sub" || userInfo.Name != "Access Token User" {
		t.Fatalf("expected access-token fallback, got %+v", userInfo)
	}
}

func TestResolveUserInfoReturnsEmptyProfileWithoutError(t *testing.T) {
	middleware, store := newUserInfoTestMiddleware("id_token")
	defer store.Close()

	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer userinfoServer.Close()
	middleware.providerConfig.UserInfoEndpoint = userinfoServer.URL

	userInfo := middleware.resolveUserInfo(&TokenResponse{
		AccessToken: "not-a-jwt",
		IDToken:     "not-a-jwt",
	})
	if userInfo == nil {
		t.Fatal("expected a non-nil user info")
	}
	if userInfo.Sub != "" || userInfo.Name != "" || userInfo.Email != "" {
		t.Fatalf("expected an empty profile, got %+v", userInfo)
	}
}

func TestResolveUserInfoUsesUserinfoByDefault(t *testing.T) {
	middleware, store := newUserInfoTestMiddleware("")
	defer store.Close()

	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"sub":"userinfo-sub","name":"User Info"}`)
	}))
	defer userinfoServer.Close()
	middleware.providerConfig.UserInfoEndpoint = userinfoServer.URL

	userInfo := middleware.resolveUserInfo(&TokenResponse{
		AccessToken: "access-token",
		IDToken:     testJWT(testIDTokenClaims()),
	})
	if userInfo.Sub != "userinfo-sub" {
		t.Fatalf("expected userinfo source, got %+v", userInfo)
	}
}

func TestHandleCallbackDoesNotFailWhenUserInfoCannotBeResolved(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"not-a-jwt","id_token":"not-a-jwt","expires_in":300}`)
	}))
	defer tokenServer.Close()

	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer userinfoServer.Close()

	store := NewMemorySessionStore()
	defer store.Close()

	middleware := newTestOIDCMiddleware(&config.Config{
		OIDCClientID:       testClientID,
		OIDCUserInfoSource: "id_token",
		SessionCookieName:  "test-session",
		SessionMaxAge:      300,
	}, store, tokenServer.URL)
	middleware.providerConfig.Issuer = testIssuer
	middleware.providerConfig.UserInfoEndpoint = userinfoServer.URL
	middleware.stateRedirect["state"] = "/"

	request := httptest.NewRequest(http.MethodGet, "/oidc/callback?state=state&code=code", nil)
	recorder := httptest.NewRecorder()
	middleware.HandleCallback(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected redirect after successful login, got %d: %s", recorder.Code, recorder.Body.String())
	}
	cookie := findCookie(recorder.Result().Cookies(), "test-session")
	if cookie == nil {
		t.Fatal("expected a session cookie")
	}
	session, err := store.Get(cookie.Value)
	if err != nil {
		t.Fatalf("expected a stored session: %v", err)
	}
	if session.UserInfo == nil {
		t.Fatal("expected a user info object in the session")
	}
	if session.UserInfo.Sub != "" {
		t.Fatalf("expected an empty profile, got %+v", session.UserInfo)
	}
}
