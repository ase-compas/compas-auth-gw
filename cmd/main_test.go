package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ase-compas/compas-auth-proxy/internal/middleware"
)

func TestUserInfoHandlerAlwaysReturnsRolesArray(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  []any
	}{
		{name: "no roles", want: []any{}},
		{name: "roles", roles: []string{"fleet-mgmt:reader", "fleet-mgmt:writer"}, want: []any{"fleet-mgmt:reader", "fleet-mgmt:writer"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userInfo := &middleware.UserInfo{
				Sub:   "user-id",
				Name:  "Jane Doe",
				Email: "jane@example.com",
				Roles: tt.roles,
			}
			request := httptest.NewRequest(http.MethodGet, "/auth/userinfo", nil)
			request = request.WithContext(middleware.SetUserInContext(context.Background(), userInfo))

			recorder := httptest.NewRecorder()
			newUserInfoHandler().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}

			var response map[string]any
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			roles, ok := response["roles"].([]any)
			if !ok {
				t.Fatalf("roles is %T, want an array", response["roles"])
			}
			if len(roles) != len(tt.want) {
				t.Fatalf("roles = %v, want %v", roles, tt.want)
			}
			for i := range tt.want {
				if roles[i] != tt.want[i] {
					t.Fatalf("roles = %v, want %v", roles, tt.want)
				}
			}
		})
	}
}

func TestUserInfoHandlerRejectsUnauthenticatedRequests(t *testing.T) {
	recorder := httptest.NewRecorder()
	newUserInfoHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/userinfo", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLoggingMiddlewareHealthRequestsAreDebugOnly(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		logLevel      string
		wantLogOutput bool
		wantDebug     bool
	}{
		{name: "health at info is suppressed", path: "/health", logLevel: "info"},
		{name: "health at debug is logged", path: "/health", logLevel: "debug", wantLogOutput: true, wantDebug: true},
		{name: "other requests remain logged", path: "/auth/userinfo", logLevel: "info", wantLogOutput: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			previousOutput := log.Writer()
			log.SetOutput(&output)
			t.Cleanup(func() { log.SetOutput(previousOutput) })

			handler := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}), tt.logLevel)
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tt.path, nil))

			gotOutput := output.String()
			if (gotOutput != "") != tt.wantLogOutput {
				t.Fatalf("log output = %q, want output present %t", gotOutput, tt.wantLogOutput)
			}
			if strings.Contains(gotOutput, "DEBUG ") != tt.wantDebug {
				t.Fatalf("debug marker in output = %t, want %t; output: %q", strings.Contains(gotOutput, "DEBUG "), tt.wantDebug, gotOutput)
			}
		})
	}
}
