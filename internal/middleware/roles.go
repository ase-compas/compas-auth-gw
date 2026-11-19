package middleware

import (
	"encoding/json"
	"fmt"
	"log"
)

const (
	rolesClaim          = "roles"
	realmAccessClaim    = "realm_access"
	resourceAccessClaim = "resource_access"
)

// extractRolesFromAccessToken returns the roles carried by the access token.
// It never fails a login: disabled extraction, an opaque token, a malformed
// token, or an unexpected claim shape all result in an empty slice.
func (m *OIDCMiddleware) extractRolesFromAccessToken(accessToken string) []string {
	if m.config == nil || !m.config.OIDCRolesEnabled {
		return []string{}
	}

	roles, err := rolesFromAccessToken(accessToken, m.config.OIDCKeycloakClientID)
	if err != nil {
		log.Printf("Warning: cannot extract roles from access token: %v", err)
		return []string{}
	}

	return roles
}

// rolesFromAccessToken reads the provider-specific role claims from a JWT
// access token:
//
//   - Entra ID: top-level "roles" claim (app roles defined on the API app).
//   - Keycloak: "realm_access.roles" and, when keycloakClientID is set,
//     "resource_access.<keycloakClientID>.roles".
//
// Signature validation is intentionally not performed here. The token is
// received directly from the provider token endpoint over TLS, and these roles
// are informational only: the upstream API still validates the token before
// making authorization decisions.
func rolesFromAccessToken(accessToken, keycloakClientID string) ([]string, error) {
	payload, err := decodeJWTPayload(accessToken)
	if err != nil {
		return nil, err
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("decode access token claims: %w", err)
	}

	var roles []string
	roles = append(roles, stringValues(claims[rolesClaim])...)

	if realmAccess, ok := claims[realmAccessClaim].(map[string]any); ok {
		roles = append(roles, stringValues(realmAccess[rolesClaim])...)
	}

	if keycloakClientID != "" {
		if resourceAccess, ok := claims[resourceAccessClaim].(map[string]any); ok {
			if clientAccess, ok := resourceAccess[keycloakClientID].(map[string]any); ok {
				roles = append(roles, stringValues(clientAccess[rolesClaim])...)
			}
		}
	}

	return uniqueStrings(roles), nil
}

// stringValues converts a JSON claim value into role names. Only string arrays
// and single strings are accepted; other shapes are ignored so that an
// unexpected claim cannot break login or profile extraction.
func stringValues(value any) []string {
	switch typed := value.(type) {
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return values
	case []string:
		return typed
	case string:
		return []string{typed}
	default:
		return nil
	}
}

// uniqueStrings removes empty and duplicate role names while preserving the
// order in which the provider supplied them. Role values are case-sensitive.
func uniqueStrings(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}

	return unique
}
