package inbound

import (
	"kong-plugins/internal/adapter/outbound"
	"kong-plugins/internal/domain/entity"
	"time"

	"kong-plugins/internal/usecase"

	"github.com/spf13/viper"
)

func GetAuthChecker(config entity.AuthConfig) *AuthChecker {
	authVerifier := outbound.NewAuthVerifierRestImpl()
	cache := outbound.NewMemoryCache(time.Second*time.Duration(viper.GetInt("auth_cache.ttl")), time.Second*time.Duration(viper.GetInt("auth_cache.cleanup_interval")))
	kongGW := outbound.NewKongGatewayImpl()
	uc := usecase.NewCheckAuth(&config, authVerifier, cache, kongGW)
	authChecker := NewAuthChecker(uc)
	return authChecker
}
