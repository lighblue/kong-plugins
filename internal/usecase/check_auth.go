package usecase

import (
	"context"
	"log"

	"kong-plugins/internal/domain/entity"

	"github.com/Kong/go-pdk"
)

type CheckAuth struct{}

func NewCheckAuth() *CheckAuth {
	return &CheckAuth{}
}

func (CheckAuth) Execute(_ context.Context, _ entity.AuthConfig, kong *pdk.PDK) {
	log.Print("check auth placeholder: allowing request")
}
