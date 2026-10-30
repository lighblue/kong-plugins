package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kong/go-pdk/test"
)

// Scenario: run the plugin with missing, valid, and denied credentials; expect the matching response status and error code.
func TestAuthHeader(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		headers    http.Header
		authStatus int
		wantStatus int
	}{
		// Scenario: omit Authorization; expect 400 and ERR0002.
		{
			name:       "missing header",
			headerName: "Authorization",
			wantStatus: http.StatusBadRequest,
		},
		// Scenario: send an empty Authorization value; expect 400 and ERR0002.
		{
			name:       "empty header",
			headerName: "Authorization",
			headers:    http.Header{"Authorization": {""}},
			wantStatus: http.StatusBadRequest,
		},
		// Scenario: leave the configured credential header name unset; expect 400 and ERR0002.
		{
			name:       "unconfigured header name",
			wantStatus: http.StatusBadRequest,
		},
		// Scenario: send a credential accepted by the auth server; expect the request to continue.
		{
			name:       "present header",
			headerName: "Authorization",
			headers:    http.Header{"Authorization": {"Bearer token"}},
		},
		// Scenario: send a credential rejected with 403; expect 401 and ERR0004.
		{
			name:       "auth server rejects",
			headerName: "Authorization",
			headers:    http.Header{"Authorization": {"Bearer token"}},
			authStatus: http.StatusForbidden,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.authStatus != 0 {
					w.WriteHeader(tc.authStatus)
				}
			}))
			defer authServer.Close()

			env, err := test.New(t, test.Request{
				Method:  http.MethodGet,
				Url:     "http://example.com/",
				Headers: tc.headers,
			})
			if err != nil {
				t.Fatal(err)
			}

			config := NewAuthConfig().(*Config)
			config.RequestHeaderName = tc.headerName
			config.AuthServerURL = authServer.URL
			env.DoAccess(config)
			if env.ClientRes.Status != tc.wantStatus {
				t.Fatalf("response status = %d, want %d", env.ClientRes.Status, tc.wantStatus)
			}
			if gotRunning := env.IsRunning(); gotRunning != (tc.wantStatus == 0) {
				t.Fatalf("request running = %t, want %t", gotRunning, tc.wantStatus == 0)
			}
			if tc.wantStatus != 0 {
				if got := env.ClientRes.Headers.Get("Content-Type"); got != "application/json; charset=utf-8" {
					t.Fatalf("content type = %q, want JSON", got)
				}
				var body map[string]interface{}
				if err := json.Unmarshal(env.ClientRes.Body, &body); err != nil {
					t.Fatalf("decode response body: %v", err)
				}
				message, ok := body["message"].(string)
				wantCode := "ERR0002"
				if tc.authStatus != 0 {
					wantCode = "ERR0004"
				}
				if len(body) != 2 || !ok || message == "" || body["code"] != wantCode {
					t.Fatalf("response body = %v, want message and code", body)
				}
			}
		})
	}
}

// Scenario: decode Kong plugin JSON containing a response-field mapping; expect the source field and header name to be retained.
func TestConfigAcceptsKongExtraHeaderMappings(t *testing.T) {
	config := NewAuthConfig().(*Config)
	if err := json.Unmarshal([]byte(`{
		"auth_server_url": "http://auth.example.com/check",
		"request_header_name": "Authorization",
		"extra_header_mappings": [
			{"source_field": "jwt", "name": "X-Auth-Token"}
		]
	}`), config); err != nil {
		t.Fatal(err)
	}
	if len(config.ExtraHeaderMappings) != 1 || config.ExtraHeaderMappings[0].SourceField != "jwt" || config.ExtraHeaderMappings[0].Name != "X-Auth-Token" {
		t.Fatalf("response header mappings = %#v, want jwt -> X-Auth-Token", config.ExtraHeaderMappings)
	}
}
