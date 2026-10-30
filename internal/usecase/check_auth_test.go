package usecase

import (
	"context"
	"errors"
	"net"
	"net/http"
	"reflect"
	"testing"

	"kong-plugins/internal/domain/entity"

	"github.com/Kong/go-pdk"
	"github.com/Kong/go-pdk/bridge"
	"github.com/Kong/go-pdk/log"
	"github.com/Kong/go-pdk/request"
	pdktest "github.com/Kong/go-pdk/test"
)

type stubAuthVerifier struct {
	response map[string]interface{}
	err      error
	calls    int
	ctx      context.Context
	url      string
	headers  map[string]string
}

func (v *stubAuthVerifier) Verify(ctx context.Context, url string, headers map[string]string) (map[string]interface{}, error) {
	v.calls++
	v.ctx = ctx
	v.url = url
	v.headers = headers
	return v.response, v.err
}

type stubCache struct {
	value    interface{}
	getErr   error
	setErr   error
	getKeys  []string
	setKeys  []string
	setValue interface{}
}

func (c *stubCache) Get(key string) (interface{}, error) {
	c.getKeys = append(c.getKeys, key)
	return c.value, c.getErr
}

func (c *stubCache) Set(key string, value interface{}) error {
	c.setKeys = append(c.setKeys, key)
	c.setValue = value
	if c.setErr == nil {
		c.value = value
	}
	return c.setErr
}

type rejection struct {
	status int
	resp   entity.ErrorResponse
}

type stubKongGateway struct {
	appended   map[string][]string
	appendErr  error
	rejections []rejection
}

func (g *stubKongGateway) GetRequestHeader(_ *pdk.PDK, _ string) (string, error) {
	return "", errors.New("unexpected GetRequestHeader call")
}

func (g *stubKongGateway) AppendRequestHeaders(_ *pdk.PDK, headers map[string][]string) error {
	g.appended = headers
	return g.appendErr
}

func (g *stubKongGateway) RejectRequest(_ *pdk.PDK, status int, resp entity.ErrorResponse) error {
	g.rejections = append(g.rejections, rejection{status: status, resp: resp})
	return nil
}

type accessFunc func(*pdk.PDK)

func (f accessFunc) Access(kong *pdk.PDK) { f(kong) }

func executeForTest(t *testing.T, uc *CheckAuth, headers http.Header) error {
	t.Helper()
	env, err := pdktest.New(t, pdktest.Request{
		Method:  http.MethodGet,
		Url:     "http://example.com/",
		Headers: headers,
	})
	if err != nil {
		t.Fatal(err)
	}
	var executeErr error
	env.DoAccess(accessFunc(func(kong *pdk.PDK) {
		executeErr = uc.Execute(context.Background(), kong)
	}))
	return executeErr
}

func testAuthConfig() *entity.AuthConfig {
	return &entity.AuthConfig{
		AuthServerURL:     "http://auth.example.com/check",
		RequestHeaderName: "Authorization",
	}
}

// Scenario: authenticate a request with mapped response fields; expect a remote check, cache write, and selected upstream headers.
func TestExecuteVerifiesAndAppendsMappedResponseFields(t *testing.T) {
	config := testAuthConfig()
	config.ExtraHeaderMappings = []entity.ExtraHeaderMapping{
		{SourceField: "jwt", Name: "X-Auth-Token"},
		{SourceField: "level", Name: "X-Auth-Level"},
		{SourceField: "missing", Name: "X-Missing"},
	}
	response := map[string]interface{}{"jwt": "signed-token", "level": float64(3)}
	verifier := &stubAuthVerifier{response: response}
	cache := &stubCache{}
	gw := &stubKongGateway{}
	uc := NewCheckAuth(config, verifier, cache, gw)

	if err := executeForTest(t, uc, http.Header{"Authorization": {"Bearer abc"}}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if verifier.calls != 1 || verifier.url != config.AuthServerURL || !reflect.DeepEqual(verifier.headers, map[string]string{"Authorization": "Bearer abc"}) {
		t.Fatalf("Verify() calls = %d, url = %q, headers = %#v", verifier.calls, verifier.url, verifier.headers)
	}
	if verifier.ctx == nil {
		t.Fatal("Verify() did not receive a context")
	}
	if !reflect.DeepEqual(cache.getKeys, []string{"Bearer abc"}) || !reflect.DeepEqual(cache.setKeys, []string{"Bearer abc"}) || !reflect.DeepEqual(cache.setValue, response) {
		t.Fatalf("cache get keys = %v, set keys = %v, value = %#v", cache.getKeys, cache.setKeys, cache.setValue)
	}
	wantHeaders := map[string][]string{
		"X-Auth-Token": {"signed-token"},
		"X-Auth-Level": {"3"},
	}
	if !reflect.DeepEqual(gw.appended, wantHeaders) || len(gw.rejections) != 0 {
		t.Fatalf("appended headers = %#v, rejections = %#v", gw.appended, gw.rejections)
	}
}

// Scenario: authenticate with a cached response; expect no remote call and the cached field in upstream headers.
func TestExecuteUsesCachedResponse(t *testing.T) {
	config := testAuthConfig()
	config.ExtraHeaderMappings = []entity.ExtraHeaderMapping{{SourceField: "jwt", Name: "X-Auth-Token"}}
	verifier := &stubAuthVerifier{err: errors.New("should not call verifier")}
	cache := &stubCache{value: map[string]interface{}{"jwt": "cached-token"}}
	gw := &stubKongGateway{}
	uc := NewCheckAuth(config, verifier, cache, gw)

	if err := executeForTest(t, uc, http.Header{"Authorization": {"Bearer abc"}}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if verifier.calls != 0 || len(cache.setKeys) != 0 || !reflect.DeepEqual(cache.getKeys, []string{"Bearer abc"}) {
		t.Fatalf("verifier calls = %d, cache get keys = %v, set keys = %v", verifier.calls, cache.getKeys, cache.setKeys)
	}
	if !reflect.DeepEqual(gw.appended, map[string][]string{"X-Auth-Token": {"cached-token"}}) || len(gw.rejections) != 0 {
		t.Fatalf("appended headers = %#v, rejections = %#v", gw.appended, gw.rejections)
	}
}

// Scenario: the configured credential header is missing or empty; expect a 400 rejection before verification.
func TestExecuteRejectsMissingOrEmptyHeader(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers http.Header
	}{
		// Scenario: omit Authorization; expect 400 without cache or verifier calls.
		{name: "missing"},
		// Scenario: send empty Authorization; expect 400 without cache or verifier calls.
		{name: "empty", headers: http.Header{"Authorization": {""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier := &stubAuthVerifier{}
			cache := &stubCache{}
			gw := &stubKongGateway{}
			uc := NewCheckAuth(testAuthConfig(), verifier, cache, gw)

			if err := executeForTest(t, uc, tc.headers); err == nil {
				t.Fatal("Execute() error = nil, want missing-header error")
			}
			if verifier.calls != 0 || len(cache.getKeys) != 0 || len(cache.setKeys) != 0 {
				t.Fatalf("verifier calls = %d, cache get keys = %v, set keys = %v", verifier.calls, cache.getKeys, cache.setKeys)
			}
			if len(gw.rejections) != 1 || gw.rejections[0].status != http.StatusBadRequest || gw.rejections[0].resp.Code != "ERR0002" {
				t.Fatalf("rejections = %#v, want one 400/ERR0002", gw.rejections)
			}
		})
	}
}

// Scenario: Kong fails to read the credential header; expect a 400 rejection without remote verification.
func TestExecuteRejectsRequestHeaderReadError(t *testing.T) {
	conn, peer := net.Pipe()
	_ = conn.Close()
	_ = peer.Close()
	kong := &pdk.PDK{
		Request: request.Request{PdkBridge: bridge.New(conn)},
		Log:     log.Log{PdkBridge: bridge.New(conn)},
	}
	verifier := &stubAuthVerifier{}
	cache := &stubCache{}
	gw := &stubKongGateway{}
	uc := NewCheckAuth(testAuthConfig(), verifier, cache, gw)

	if err := uc.Execute(context.Background(), kong); err == nil {
		t.Fatal("Execute() error = nil, want request-header read error")
	}
	if verifier.calls != 0 || len(cache.getKeys) != 0 {
		t.Fatalf("verifier calls = %d, cache get keys = %v", verifier.calls, cache.getKeys)
	}
	if len(gw.rejections) != 1 || gw.rejections[0].status != http.StatusBadRequest || gw.rejections[0].resp.Code != "ERR0001" {
		t.Fatalf("rejections = %#v, want one 400/ERR0001", gw.rejections)
	}
}

// Scenario: the cache cannot provide a usable auth response; expect remote verification and a cache write.
func TestVerifyFallsBackToRemote(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cached   interface{}
		cacheErr error
	}{
		// Scenario: find no cached response; expect remote verification and cache write.
		{name: "cache miss"},
		// Scenario: fail to read the cache; expect remote verification and cache write.
		{name: "cache read error", cacheErr: errors.New("cache unavailable")},
		// Scenario: find an unexpected cached value type; expect remote verification and cache write.
		{name: "wrong cached type", cached: "not a response map"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := map[string]interface{}{"jwt": "remote-token"}
			verifier := &stubAuthVerifier{response: response}
			cache := &stubCache{value: tc.cached, getErr: tc.cacheErr}
			uc := NewCheckAuth(testAuthConfig(), verifier, cache, &stubKongGateway{})

			got, err := uc.verify(context.Background(), "Bearer abc")
			if err != nil || !reflect.DeepEqual(got, response) {
				t.Fatalf("verify() = %#v, %v; want %#v, nil", got, err, response)
			}
			if verifier.calls != 1 || !reflect.DeepEqual(cache.setKeys, []string{"Bearer abc"}) || !reflect.DeepEqual(cache.setValue, response) {
				t.Fatalf("verifier calls = %d, cache set keys = %v, value = %#v", verifier.calls, cache.setKeys, cache.setValue)
			}
		})
	}
}

// Scenario: verify the same credential twice; expect one remote call followed by a cache hit.
func TestVerifyReusesResponseAfterCacheWrite(t *testing.T) {
	response := map[string]interface{}{"jwt": "remote-token"}
	verifier := &stubAuthVerifier{response: response}
	cache := &stubCache{}
	uc := NewCheckAuth(testAuthConfig(), verifier, cache, &stubKongGateway{})
	for i := 0; i < 2; i++ {
		got, err := uc.verify(context.Background(), "Bearer abc")
		if err != nil || !reflect.DeepEqual(got, response) {
			t.Fatalf("verify() call %d = %#v, %v; want %#v, nil", i+1, got, err, response)
		}
	}
	if verifier.calls != 1 || len(cache.getKeys) != 2 || len(cache.setKeys) != 1 {
		t.Fatalf("verifier calls = %d, cache get keys = %v, set keys = %v", verifier.calls, cache.getKeys, cache.setKeys)
	}
}

// Scenario: remote verification fails; expect its error and no cache write.
func TestVerifyDoesNotCacheRemoteFailure(t *testing.T) {
	wantErr := errors.New("auth server rejected request")
	verifier := &stubAuthVerifier{err: wantErr}
	cache := &stubCache{}
	uc := NewCheckAuth(testAuthConfig(), verifier, cache, &stubKongGateway{})

	got, err := uc.verify(context.Background(), "Bearer abc")
	if got != nil || !errors.Is(err, wantErr) || len(cache.setKeys) != 0 {
		t.Fatalf("verify() = %#v, %v, cache set keys = %v; want nil, remote error, no cache write", got, err, cache.setKeys)
	}
}

// Scenario: caching a successful remote response fails; expect authentication to remain successful.
func TestVerifyCacheWriteErrorDoesNotDiscardRemoteSuccess(t *testing.T) {
	response := map[string]interface{}{"jwt": "remote-token"}
	cache := &stubCache{setErr: errors.New("cache unavailable")}
	uc := NewCheckAuth(testAuthConfig(), &stubAuthVerifier{response: response}, cache, &stubKongGateway{})

	got, err := uc.verify(context.Background(), "Bearer abc")
	if err != nil || !reflect.DeepEqual(got, response) {
		t.Fatalf("verify() = %#v, %v; want %#v, nil", got, err, response)
	}
}

// Scenario: Kong fails to set upstream headers; expect a 500 rejection with the gateway error.
func TestExecuteRejectsUpstreamHeaderWriteError(t *testing.T) {
	wantErr := errors.New("cannot set upstream headers")
	gw := &stubKongGateway{appendErr: wantErr}
	uc := NewCheckAuth(testAuthConfig(), &stubAuthVerifier{response: map[string]interface{}{}}, &stubCache{}, gw)

	if err := executeForTest(t, uc, http.Header{"Authorization": {"Bearer abc"}}); !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
	if len(gw.rejections) != 1 || gw.rejections[0].status != http.StatusInternalServerError || gw.rejections[0].resp.Code != "ERR0003" {
		t.Fatalf("rejections = %#v, want one 500/ERR0003", gw.rejections)
	}
}

// Scenario: the auth server rejects a credential; expect a client-error rejection without caching.
func TestExecuteRejectsRemoteAuthFailure(t *testing.T) {
	wantErr := errors.New("auth server returned HTTP 403")
	verifier := &stubAuthVerifier{err: wantErr}
	cache := &stubCache{}
	gw := &stubKongGateway{}
	uc := NewCheckAuth(testAuthConfig(), verifier, cache, gw)

	if err := executeForTest(t, uc, http.Header{"Authorization": {"Bearer abc"}}); !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
	if len(cache.setKeys) != 0 {
		t.Fatalf("cache set keys = %v, want no write", cache.setKeys)
	}
	if len(gw.rejections) != 1 || gw.rejections[0].status < 400 || gw.rejections[0].status >= 500 {
		t.Fatalf("rejections = %#v, want one 40x response", gw.rejections)
	}
}
