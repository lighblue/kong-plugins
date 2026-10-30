package inbound

import (
	"context"
	"fmt"
	"kong-plugins/internal/usecase"
	"time"

	"github.com/Kong/go-pdk"
	"github.com/spf13/viper"
)

type AuthChecker struct {
	checkAuthUseCase *usecase.CheckAuth
}

func NewAuthChecker(checkAuthUseCase *usecase.CheckAuth) *AuthChecker {
	return &AuthChecker{
		checkAuthUseCase: checkAuthUseCase,
	}
}

func (conf *AuthChecker) Check(kong *pdk.PDK) {
	kong.Log.Info("checking auth")

	cxt, cancel := context.WithTimeout(context.Background(), getTimeout())
	defer cancel()

	err := conf.checkAuthUseCase.Execute(cxt, kong)
	if err != nil {
		_ = kong.Log.Err(fmt.Sprintf("auth check failed: %v", err))
		return
	}
}

func getTimeout() time.Duration {
	timeout := viper.GetInt("auth_check.timeout")
	if timeout == 0 {
		timeout = 10
	}
	return time.Duration(timeout) * time.Second
}
