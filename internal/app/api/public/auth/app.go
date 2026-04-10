package auth

import (
	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/token"
)

type Implementation struct {
	authSrv     *auth.Service
	tokenSrv    *token.Service
	frontendURL string
}

func New(
	authSrv *auth.Service,
	tokenSrv *token.Service,
	frontendURL string,
) *Implementation {
	return &Implementation{
		authSrv:     authSrv,
		tokenSrv:    tokenSrv,
		frontendURL: frontendURL,
	}
}
