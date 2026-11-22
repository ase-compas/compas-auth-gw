package middleware

import (
	"reflect"
	"testing"

	"github.com/ase-compas/compas-auth-proxy/internal/config"
)

func TestRolesFromAccessToken(t *testing.T) {
	tests := []struct {
		name             string
		claims           map[string]any
		keycloakClientID string
		want             []string
	}{
		{
			name: "Entra ID top-level roles",
			claims: map[string]any{
				"sub":   "user-id",
				"roles": []string{"fleet-mgmt:reader", "fleet-mgmt:writer"},
			},
			want: []string{"fleet-mgmt:reader", "fleet-mgmt:writer"},
		},
		{
			name: "Keycloak realm and configured client roles",
			claims: map[string]any{
				"realm_access": map[string]any{
					"roles": []any{"offline_access", "compas-user"},
				},
				"resource_access": map[string]any{
					"fleet-api": map[string]any{
						"roles": []any{"fleet-mgmt:reader"},
					},
					"other-client": map[string]any{
						"roles": []any{"must-not-leak"},
					},
				},
			},
			keycloakClientID: "fleet-api",
			want:             []string{"offline_access", "compas-user", "fleet-mgmt:reader"},
		},
		{
			name: "Keycloak client roles require a configured client ID",
			claims: map[string]any{
				"realm_access": map[string]any{
					"roles": []any{"compas-user"},
				},
				"resource_access": map[string]any{
					"fleet-api": map[string]any{
						"roles": []any{"fleet-mgmt:reader"},
					},
				},
			},
			want: []string{"compas-user"},
		},
		{
			name: "duplicate roles are removed in provider order",
			claims: map[string]any{
				"roles": []any{"fleet-mgmt:reader", "fleet-mgmt:reader"},
				"realm_access": map[string]any{
					"roles": []any{"fleet-mgmt:writer", "fleet-mgmt:reader"},
				},
			},
			want: []string{"fleet-mgmt:reader", "fleet-mgmt:writer"},
		},
		{
			name: "unexpected role shapes are ignored",
			claims: map[string]any{
				"roles":        "single-role",
				"realm_access": "not-an-object",
				"resource_access": map[string]any{
					"fleet-api": "not-an-object",
				},
			},
			keycloakClientID: "fleet-api",
			want:             []string{"single-role"},
		},
		{
			name: "token without roles",
			claims: map[string]any{
				"sub": "user-id",
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rolesFromAccessToken(testJWT(tt.claims), tt.keycloakClientID)
			if err != nil {
				t.Fatalf("rolesFromAccessToken() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("rolesFromAccessToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRolesFromAccessTokenRejectsInvalidTokens(t *testing.T) {
	tokens := []string{
		"opaque-access-token",
		"not.a.jwt.with.too.many.parts",
		"eyJhbGciOiJub25lIn0.!!!.",
	}

	for _, token := range tokens {
		if _, err := rolesFromAccessToken(token, ""); err == nil {
			t.Errorf("rolesFromAccessToken(%q) expected an error", token)
		}
	}
}

func TestExtractRolesFromAccessTokenHandlesDisabledAndOpaqueTokens(t *testing.T) {
	token := testJWT(map[string]any{
		"roles": []string{"fleet-mgmt:reader"},
	})

	disabled := &OIDCMiddleware{config: &config.Config{OIDCRolesEnabled: false}}
	if got := disabled.extractRolesFromAccessToken(token); len(got) != 0 {
		t.Fatalf("disabled extraction returned roles: %v", got)
	}

	enabled := &OIDCMiddleware{config: &config.Config{OIDCRolesEnabled: true}}
	if got := enabled.extractRolesFromAccessToken(token); !reflect.DeepEqual(got, []string{"fleet-mgmt:reader"}) {
		t.Fatalf("enabled extraction returned %v", got)
	}
	if got := enabled.extractRolesFromAccessToken("opaque-access-token"); len(got) != 0 {
		t.Fatalf("opaque token returned roles: %v", got)
	}
}
