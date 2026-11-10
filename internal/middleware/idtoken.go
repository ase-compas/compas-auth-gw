package middleware

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// tokenAudience accepts the JWT "aud" claim in both forms used by OIDC
// providers: a single string (Entra ID) or an array of strings (Keycloak).
type tokenAudience []string

func (a *tokenAudience) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*a = tokenAudience{single}
		return nil
	}

	var multiple []string
	if err := json.Unmarshal(data, &multiple); err != nil {
		return fmt.Errorf("aud must be a string or an array of strings")
	}
	*a = tokenAudience(multiple)
	return nil
}

func (a tokenAudience) contains(value string) bool {
	for _, audience := range a {
		if audience == value {
			return true
		}
	}
	return false
}

// tokenClaims contains the claims needed for user info. It is used for both
// ID tokens and access tokens; not every provider populates every field.
type tokenClaims struct {
	Issuer            string        `json:"iss"`
	Audience          tokenAudience `json:"aud"`
	Subject           string        `json:"sub"`
	Expiry            int64         `json:"exp"`
	IssuedAt          int64         `json:"iat"`
	Name              string        `json:"name"`
	Email             string        `json:"email"`
	PreferredUsername string        `json:"preferred_username"`
	OID               string        `json:"oid"`
	TID               string        `json:"tid"`
}

// parseJWTClaims extracts claims from a signed or unsigned JWT payload.
//
// Signature validation is intentionally not performed here. The token is
// received directly from the provider's token endpoint over TLS, which OIDC
// Core 3.1.3.7 permits as an alternative to re-validating the signature.
// Claim validation (iss, aud, exp, sub) is still performed for ID tokens. A
// future hardening step should verify signatures against the provider JWKS.
func parseJWTClaims(token string) (*tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("token is not a JWT")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some providers emit padded base64url despite the JWT spec. Retry
		// with padding before giving up.
		padded := parts[1] + strings.Repeat("=", (4-len(parts[1])%4)%4)
		payload, err = base64.URLEncoding.DecodeString(padded)
		if err != nil {
			return nil, fmt.Errorf("decode JWT payload: %w", err)
		}
	}

	var claims tokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("decode JWT claims: %w", err)
	}

	return &claims, nil
}

func userInfoFromClaims(claims *tokenClaims) *UserInfo {
	sub := claims.Subject
	if sub == "" {
		// Entra ID access tokens always carry oid; use it as a stable fallback
		// when sub is absent.
		sub = claims.OID
	}

	return &UserInfo{
		Sub:               sub,
		Name:              claims.Name,
		Email:             claims.Email,
		PreferredUsername: claims.PreferredUsername,
		OID:               claims.OID,
		TID:               claims.TID,
	}
}

func (m *OIDCMiddleware) validateIDTokenClaims(claims *tokenClaims) error {
	if claims.Subject == "" {
		return fmt.Errorf("id token has no sub claim")
	}
	if claims.Expiry == 0 {
		return fmt.Errorf("id token has no exp claim")
	}
	if !time.Now().Before(time.Unix(claims.Expiry, 0)) {
		return fmt.Errorf("id token is expired")
	}
	if m.providerConfig != nil && m.providerConfig.Issuer != "" && claims.Issuer != m.providerConfig.Issuer {
		return fmt.Errorf("id token issuer %q does not match provider issuer %q", claims.Issuer, m.providerConfig.Issuer)
	}
	if m.config != nil && m.config.OIDCClientID != "" && !claims.Audience.contains(m.config.OIDCClientID) {
		return fmt.Errorf("id token audience does not contain client ID")
	}
	return nil
}

func (m *OIDCMiddleware) userInfoFromIDToken(idToken string) (*UserInfo, error) {
	claims, err := parseJWTClaims(idToken)
	if err != nil {
		return nil, err
	}
	if err := m.validateIDTokenClaims(claims); err != nil {
		return nil, err
	}
	return userInfoFromClaims(claims), nil
}

func (m *OIDCMiddleware) userInfoFromAccessToken(accessToken string) (*UserInfo, error) {
	claims, err := parseJWTClaims(accessToken)
	if err != nil {
		return nil, err
	}

	userInfo := userInfoFromClaims(claims)
	if userInfo.Sub == "" {
		return nil, fmt.Errorf("access token has no sub or oid claim")
	}
	return userInfo, nil
}

func hasProfileClaims(userInfo *UserInfo) bool {
	return userInfo != nil && userInfo.Sub != "" &&
		(userInfo.Name != "" || userInfo.Email != "" || userInfo.PreferredUsername != "")
}

// resolveUserInfo resolves the user profile without failing the login.
//
// Behavior by oidc.userinfo_source:
//   - userinfo: call the provider userinfo endpoint (backwards compatible).
//   - id_token: use the ID token when it contains a valid sub; fall back to
//     userinfo and then to access-token claims.
//   - auto: prefer an ID token that also contains at least one profile claim;
//     otherwise try userinfo and then access-token claims.
//
// Missing optional claims, an unavailable userinfo endpoint, or an unparseable
// token never produce an error. The proxy logs a warning and continues with
// whatever information is available.
func (m *OIDCMiddleware) resolveUserInfo(tokenResp *TokenResponse) *UserInfo {
	source := "userinfo"
	if m.config != nil && m.config.OIDCUserInfoSource != "" {
		source = m.config.OIDCUserInfoSource
	}

	var idTokenInfo *UserInfo
	var idTokenErr error

	if source == "id_token" || source == "auto" {
		if tokenResp.IDToken == "" {
			idTokenErr = fmt.Errorf("token response has no id_token")
		} else {
			idTokenInfo, idTokenErr = m.userInfoFromIDToken(tokenResp.IDToken)
			if idTokenErr == nil {
				// In id_token mode, sub is sufficient. In auto mode, prefer an
				// ID token that also contains at least one profile claim;
				// otherwise try userinfo and keep the partial ID token as a
				// final fallback.
				if source == "id_token" || hasProfileClaims(idTokenInfo) {
					return idTokenInfo
				}
				idTokenErr = fmt.Errorf("id token contains no profile claims")
			}
		}

		log.Printf("Warning: cannot use ID token for user info: %v; falling back to userinfo", idTokenErr)
	}

	userInfo, err := m.getUserInfo(tokenResp.AccessToken)
	if err == nil && userInfo != nil && userInfo.Sub != "" {
		return userInfo
	}
	if err == nil {
		err = fmt.Errorf("userinfo response has no sub claim")
	}
	log.Printf("Warning: cannot retrieve user info from userinfo endpoint: %v; falling back to access-token claims", err)

	// Preserve partial ID-token data before falling back to the access token.
	if idTokenInfo != nil && idTokenInfo.Sub != "" {
		return idTokenInfo
	}

	accessTokenInfo, accessTokenErr := m.userInfoFromAccessToken(tokenResp.AccessToken)
	if accessTokenErr == nil && accessTokenInfo != nil {
		return accessTokenInfo
	}
	log.Printf("Warning: cannot extract user info from access token: %v; continuing with empty profile", accessTokenErr)

	return &UserInfo{}
}
