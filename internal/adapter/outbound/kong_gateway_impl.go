package outbound

import (
	"encoding/json"
	"kong-plugins/internal/domain/entity"

	"github.com/Kong/go-pdk"
)

type KongGatewayImpl struct{}

func NewKongGatewayImpl() *KongGatewayImpl {
	return &KongGatewayImpl{}
}

func (gw *KongGatewayImpl) GetRequestHeader(kong *pdk.PDK, key string) (string, error) {
	return kong.Request.GetHeader(key)
}

func (gw *KongGatewayImpl) AppendRequestHeaders(kong *pdk.PDK, headers map[string][]string) error {
	return kong.ServiceRequest.SetHeaders(headers)
}

func (gw *KongGatewayImpl) RejectRequest(kong *pdk.PDK, statusCode int, resp entity.ErrorResponse) error {
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	kong.Response.Exit(statusCode, body, map[string][]string{
		"Content-Type": {"application/json; charset=utf-8"},
	})
	return nil
}
