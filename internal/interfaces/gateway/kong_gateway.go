package gateway

import (
	"kong-plugins/internal/domain/entity"

	"github.com/Kong/go-pdk"
)

type KongGateway interface {
	GetRequestHeader(kong *pdk.PDK, key string) (string, error)
	AppendRequestHeaders(kong *pdk.PDK, headers map[string][]string) error
	RejectRequest(kong *pdk.PDK, statusCode int, resp entity.ErrorResponse) error
}
