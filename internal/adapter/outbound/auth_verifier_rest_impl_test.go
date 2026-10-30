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

func TestVerifyAllows2xxWithValidOrEmptyResponse(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{name: "200", status: http.StatusOK},
		{name: "201", status: http.StatusCreated},
		{name: "204", status: http.StatusNoContent},
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

func TestVerifyRejectsUnsuccessfulOrInvalidResponses(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, wantErr: "HTTP 401"},
		{name: "server error", status: http.StatusServiceUnavailable, wantErr: "HTTP 503"},
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
