package outbound

import (
	"encoding/json"
	"net/http"
	"testing"

	"kong-plugins/internal/domain/entity"

	"github.com/Kong/go-pdk"
	pdktest "github.com/Kong/go-pdk/test"
)

type gatewayAccessFunc func(*pdk.PDK)

func (f gatewayAccessFunc) Access(kong *pdk.PDK) { f(kong) }

func newGatewayTestEnv(t *testing.T, headers http.Header) *pdktest.TestEnv {
	t.Helper()
	env, err := pdktest.New(t, pdktest.Request{
		Method:  http.MethodGet,
		Url:     "http://example.com/",
		Headers: headers,
	})
	if err != nil {
		t.Fatal(err)
	}
	return env
}

// Scenario: read a present or missing request header through the Kong adapter; expect its value or an empty string.
func TestKongGatewayGetRequestHeader(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers http.Header
		want    string
	}{
		// Scenario: read a present Authorization header; expect its credential value.
		{name: "present", headers: http.Header{"Authorization": {"Bearer token"}}, want: "Bearer token"},
		// Scenario: read an absent Authorization header; expect an empty string.
		{name: "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newGatewayTestEnv(t, tc.headers)
			var got string
			var getErr error
			env.DoAccess(gatewayAccessFunc(func(kong *pdk.PDK) {
				got, getErr = NewKongGatewayImpl().GetRequestHeader(kong, "Authorization")
			}))
			if getErr != nil || got != tc.want {
				t.Fatalf("GetRequestHeader() = %q, %v; want %q, nil", got, getErr, tc.want)
			}
		})
	}
}

// Scenario: set upstream headers through the Kong adapter; expect replacements, added values, and unrelated headers to remain correct.
func TestKongGatewayAppendRequestHeaders(t *testing.T) {
	env := newGatewayTestEnv(t, http.Header{
		"Authorization": {"old-token"},
		"X-Trace":       {"trace-id"},
	})
	var appendErr error
	env.DoAccess(gatewayAccessFunc(func(kong *pdk.PDK) {
		appendErr = NewKongGatewayImpl().AppendRequestHeaders(kong, map[string][]string{
			"Authorization": {"new-token"},
			"X-Auth-Token":  {"signed-token"},
			"X-Auth-Scope":  {"read", "write"},
		})
	}))
	if appendErr != nil {
		t.Fatal(appendErr)
	}
	if got := env.ServiceReq.Headers.Get("Authorization"); got != "new-token" {
		t.Fatalf("upstream Authorization = %q, want new-token", got)
	}
	if got := env.ServiceReq.Headers.Get("X-Auth-Token"); got != "signed-token" {
		t.Fatalf("upstream X-Auth-Token = %q, want signed-token", got)
	}
	if got := env.ServiceReq.Headers.Values("X-Auth-Scope"); len(got) != 2 || got[0] != "read" || got[1] != "write" {
		t.Fatalf("upstream X-Auth-Scope = %v, want [read write]", got)
	}
	if got := env.ServiceReq.Headers.Get("X-Trace"); got != "trace-id" {
		t.Fatalf("upstream X-Trace = %q, want preserved trace-id", got)
	}
	if got := env.ClientReq.Headers.Get("Authorization"); got != "old-token" {
		t.Fatalf("original Authorization = %q, want old-token", got)
	}
}

// Scenario: reject a request through the Kong adapter; expect a stopped request and JSON error response.
func TestKongGatewayRejectRequest(t *testing.T) {
	env := newGatewayTestEnv(t, nil)
	want := entity.ErrorResponse{Message: "authentication failed", Code: "ERR_AUTH"}
	var rejectErr error
	env.DoAccess(gatewayAccessFunc(func(kong *pdk.PDK) {
		rejectErr = NewKongGatewayImpl().RejectRequest(kong, http.StatusUnauthorized, want)
	}))
	if rejectErr != nil {
		t.Fatal(rejectErr)
	}
	if env.IsRunning() || env.ClientRes.Status != http.StatusUnauthorized {
		t.Fatalf("running = %t, status = %d; want stopped with 401", env.IsRunning(), env.ClientRes.Status)
	}
	if got := env.ClientRes.Headers.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", got)
	}
	var body entity.ErrorResponse
	if err := json.Unmarshal(env.ClientRes.Body, &body); err != nil {
		t.Fatalf("decode rejection body: %v", err)
	}
	if body != want {
		t.Fatalf("rejection body = %#v, want %#v", body, want)
	}
}
