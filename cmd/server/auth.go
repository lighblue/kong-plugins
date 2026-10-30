package main

import (
	"kong-plugins/cmd/common"
	"kong-plugins/internal/adapter/inbound"
	"kong-plugins/internal/domain/entity"
	"sync"

	"github.com/Kong/go-pdk"
	"github.com/Kong/go-pdk/server"
)

const (
	version  = "0.0.1"
	priority = 1
)

type Config struct {
	AuthServerURL       string                      `json:"auth_server_url"`
	RequestHeaderName   string                      `json:"request_header_name"`
	ExtraHeaderMappings []entity.ExtraHeaderMapping `json:"extra_header_mappings"`
	getChecker          func() *inbound.AuthChecker
}

func NewAuthConfig() interface{} {
	conf := &Config{}
	conf.getChecker = sync.OnceValue(func() *inbound.AuthChecker {
		return inbound.GetAuthChecker(conf.authConfig())
	})
	return conf
}

func (conf Config) Access(kong *pdk.PDK) {
	conf.getChecker().Check(kong)
}

func (conf Config) authConfig() entity.AuthConfig {
	return entity.AuthConfig{
		AuthServerURL:       conf.AuthServerURL,
		RequestHeaderName:   conf.RequestHeaderName,
		ExtraHeaderMappings: conf.ExtraHeaderMappings,
	}
}

func main() {
	common.LoadConfig("/configs/config.toml")
	server.StartServer(NewAuthConfig, version, priority)
}
