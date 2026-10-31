package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
