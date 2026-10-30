package outbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Scenario: POST to a successful auth server; expect forwarded headers and a decoded JSON response.
func TestVerifySendsHeadersAndDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("Authorization = %q, want Bearer token", got)
		}
		if got := r.Header.Get("X-Tenant"); got != "team-a" {
			t.Errorf("X-Tenant = %q, want team-a", got)
		}
		_, _ = w.Write([]byte(`{"jwt":"signed-token","user_id":42,"ignored":"value"}`))
	}))
	defer server.Close()

	fields, err := NewAuthVerifierRestImpl().Verify(context.Background(), server.URL, map[string]string{
		"Authorization": "Bearer token",
		"X-Tenant":      "team-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 || fields["jwt"] != "signed-token" || fields["user_id"] != float64(42) || fields["ignored"] != "value" {
		t.Fatalf("fields = %#v, want decoded auth response", fields)
	}
}

// Scenario: receive a 2xx auth response, including an empty 204; expect success and the appropriate payload.
func TestVerifyAllows2xxWithValidOrEmptyResponse(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		// Scenario: receive 200 with JSON; expect decoded auth fields.
		{name: "200", status: http.StatusOK},
		// Scenario: receive 201 with JSON; expect decoded auth fields.
		{name: "201", status: http.StatusCreated},
		// Scenario: receive 204 without a body; expect success with nil fields.
		{name: "204", status: http.StatusNoContent},
		// Scenario: receive 299 with JSON; expect decoded auth fields.
		{name: "299", status: 299},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				if tc.status != http.StatusNoContent {
					_, _ = w.Write([]byte(`{"jwt":"signed-token"}`))
				}
			}))
			defer server.Close()

			fields, err := NewAuthVerifierRestImpl().Verify(context.Background(), server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.status == http.StatusNoContent {
				if fields != nil {
					t.Fatalf("fields = %#v, want nil", fields)
				}
			} else if len(fields) != 1 || fields["jwt"] != "signed-token" {
				t.Fatalf("fields = %#v, want decoded jwt", fields)
			}
		})
	}
}

// Scenario: receive an error status or malformed JSON; expect an error and no auth fields.
func TestVerifyRejectsUnsuccessfulOrInvalidResponses(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		// Scenario: receive 401; expect an HTTP 401 error and no fields.
		{name: "unauthorized", status: http.StatusUnauthorized, wantErr: "HTTP 401"},
		// Scenario: receive 503; expect an HTTP 503 error and no fields.
		{name: "server error", status: http.StatusServiceUnavailable, wantErr: "HTTP 503"},
		// Scenario: receive malformed JSON with 200; expect a decode error and no fields.
		{name: "invalid JSON", status: http.StatusOK, body: "invalid", wantErr: "decode auth response"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			fields, err := NewAuthVerifierRestImpl().Verify(context.Background(), server.URL, nil)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tc.wantErr)
			}
			if fields != nil {
				t.Fatalf("fields = %#v, want nil on error", fields)
			}
		})
	}
}

// Scenario: the auth server redirects to another endpoint; expect the redirect to be rejected without contacting the target.
func TestVerifyDoesNotFollowRedirect(t *testing.T) {
	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected = true
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()

	_, err := NewAuthVerifierRestImpl().Verify(context.Background(), server.URL, map[string]string{"Authorization": "Bearer token"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("error = %v, want HTTP 302", err)
	}
	if redirected {
		t.Fatal("auth request followed redirect")
	}
}

// Scenario: pass an invalid URL or canceled context; expect request construction or cancellation errors.
func TestVerifyHonorsContextAndRejectsInvalidURL(t *testing.T) {
	verifier := NewAuthVerifierRestImpl()
	if _, err := verifier.Verify(context.Background(), "://invalid", nil); err == nil {
		t.Fatal("invalid auth URL was accepted")
	}

	cxt, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := verifier.Verify(cxt, "http://example.com", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

// Scenario: the auth server waits beyond the context deadline; expect the in-flight request to stop.
func TestVerifyStopsInFlightRequestAtContextDeadline(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := NewAuthVerifierRestImpl().Verify(ctx, server.URL, nil)
		result <- err
	}()

	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("auth server did not receive request before deadline")
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Verify() error = %v, want context deadline exceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Verify() did not stop after context deadline")
	}
}

// Scenario: the auth server is unreachable; expect a connection error and no decoded fields.
func TestVerifyReturnsConnectionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	fields, err := NewAuthVerifierRestImpl().Verify(context.Background(), url, nil)
	if err == nil || !strings.Contains(err.Error(), "call auth server") {
		t.Fatalf("Verify() error = %v, want connection error", err)
	}
	if fields != nil {
		t.Fatalf("Verify() fields = %#v, want nil", fields)
	}
}
