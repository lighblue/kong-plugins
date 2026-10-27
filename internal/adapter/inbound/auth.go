package inbound

import (
	"context"
	"kong-plugins/internal/domain/entity"
	"kong-plugins/internal/usecase"

	"github.com/Kong/go-pdk"
)

type AuthChecker struct {
	checkAuthUseCase *usecase.CheckAuth
}

func NewAuthChecker(checkAuthUseCase *usecase.CheckAuth) *AuthChecker {
	return &AuthChecker{
		checkAuthUseCase: checkAuthUseCase,
	}
}

func (conf AuthChecker) Check(config entity.AuthConfig, kong *pdk.PDK) {
	conf.checkAuthUseCase.Execute(context.TODO(), config, kong)
}
