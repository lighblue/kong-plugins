package gateway

import (
	"context"
)

type AuthVerifierGateway interface {
	Verify(cxt context.Context, authURL string, headers map[string]string) (map[string]interface{}, error)
}
