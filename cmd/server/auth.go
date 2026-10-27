package main

import (
	"kong-plugins/internal/domain/entity"
	"kong-plugins/internal/usecase"
	"log"
	"sync"

	"kong-plugins/internal/adapter/inbound"

	"github.com/Kong/go-pdk"
	"github.com/Kong/go-pdk/server"
)

type Config struct {
	AuthServerURL     string `json:"auth_server_url"`
	RequestHeaderName string `json:"request_header_name"`
}

var authCheckerSingleton = sync.OnceValue(newAuthChecker)

func NewAuthConfig() interface{} {
	log.Println("NewAuthConfig")
	return &Config{}
}

func (conf Config) Access(kong *pdk.PDK) {
	log.Println("auth placeholder: allowing request")
	authConfig := entity.AuthConfig(conf)
	authCheckerSingleton().Check(authConfig, kong)
}

func newAuthChecker() *inbound.AuthChecker {
	authCheckUseCase := usecase.NewCheckAuth()
	return inbound.NewAuthChecker(authCheckUseCase)
}

func main() {
	server.StartServer(NewAuthConfig, "1.0.0", 1)
}
