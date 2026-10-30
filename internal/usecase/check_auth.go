package usecase

import (
	"context"
	"fmt"
	"kong-plugins/internal/domain/entity"
	"kong-plugins/internal/interfaces"
	"kong-plugins/internal/interfaces/gateway"
	"log"
	"net/http"

	"github.com/Kong/go-pdk"
)

type CheckAuth struct {
	authConfig   *entity.AuthConfig
	authVerifier gateway.AuthVerifierGateway
	cache        interfaces.Cache
	kongGW       gateway.KongGateway
}

func NewCheckAuth(authConfig *entity.AuthConfig, authVerifier gateway.AuthVerifierGateway, cache interfaces.Cache, kongGW gateway.KongGateway) *CheckAuth {
	return &CheckAuth{
		authConfig:   authConfig,
		authVerifier: authVerifier,
		cache:        cache,
		kongGW:       kongGW,
	}
}

func (uc *CheckAuth) Execute(cxt context.Context, kong *pdk.PDK) error {
	// extract header from the request
	headerValue, err := uc.getRequestHeader(kong)
	if err != nil {
		return err
	}

	// verify the request
	resp, err := uc.verify(cxt, headerValue)
	if err != nil {
		uc.kongGW.RejectRequest(kong, http.StatusUnauthorized, entity.ErrorResponse{Code: "ERR0004", Message: "Authentication Failed"})
		return err
	}

	// append extra headers
	return uc.appendHeaders(kong, resp)
}

func (uc *CheckAuth) verify(cxt context.Context, headerValue string) (map[string]interface{}, error) {
	resp, err := uc.cache.Get(headerValue)
	if err == nil && resp != nil {
		if cachedResp, ok := resp.(map[string]interface{}); ok {
			return cachedResp, nil
		}
	}

	headers := map[string]string{uc.authConfig.RequestHeaderName: headerValue}
	respNew, err := uc.authVerifier.Verify(cxt, uc.authConfig.AuthServerURL, headers)
	if err == nil {
		// TODO: should error resp be saved???
		if err := uc.cache.Set(headerValue, respNew); err != nil {
			log.Printf("save auth response to cache: %v", err)
		}
	}

	return respNew, err
}

func (uc *CheckAuth) getRequestHeader(kong *pdk.PDK) (string, error) {
	headerName := uc.authConfig.RequestHeaderName
	value, err := kong.Request.GetHeader(headerName)
	if err != nil {
		_ = kong.Log.Err(fmt.Sprintf("read request header %q: %v", headerName, err))
		uc.kongGW.RejectRequest(kong, http.StatusBadRequest, entity.ErrorResponse{Code: "ERR0001", Message: "read request header error"})
		return value, err
	}
	if value == "" {
		msg := fmt.Sprintf("request header %q is missing or empty", headerName)
		_ = kong.Log.Err(msg)
		uc.kongGW.RejectRequest(kong, http.StatusBadRequest, entity.ErrorResponse{Code: "ERR0002", Message: msg})
		err = fmt.Errorf("request header %q is missing or empty", headerName)
	}
	return value, err
}

func (uc *CheckAuth) appendHeaders(kong *pdk.PDK, resp map[string]interface{}) error {
	extraHeaders := make(map[string][]string)
	for _, header := range uc.authConfig.ExtraHeaderMappings {
		value, ok := resp[header.SourceField]
		if !ok {
			log.Printf("auth response missing field %q", header.SourceField)
			continue
		}
		extraHeaders[header.Name] = []string{fmt.Sprint(value)}
	}

	err := uc.kongGW.AppendRequestHeaders(kong, extraHeaders)
	if err != nil {
		_ = kong.Log.Err(fmt.Sprintf("set upstream request headers: %v", err))
		uc.kongGW.RejectRequest(kong, http.StatusInternalServerError, entity.ErrorResponse{Code: "ERR0003", Message: "set request headers error"})
		return err
	}

	return nil
}
