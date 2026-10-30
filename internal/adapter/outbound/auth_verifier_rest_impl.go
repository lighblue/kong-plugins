package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type AuthVerifierRestImpl struct {
	client *http.Client
}

func NewAuthVerifierRestImpl() *AuthVerifierRestImpl {
	return &AuthVerifierRestImpl{
		client: &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (verifier *AuthVerifierRestImpl) Verify(cxt context.Context, authURL string, headers map[string]string) (map[string]interface{}, error) {
	request, err := http.NewRequestWithContext(cxt, http.MethodPost, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create auth request: %w", err)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	response, err := verifier.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call auth server: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("auth server returned HTTP %d", response.StatusCode)
	}

	decoder := json.NewDecoder(response.Body)
	var payload map[string]interface{}
	if err := decoder.Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, fmt.Errorf("decode auth response: %w", err)
	}

	return payload, nil
}
