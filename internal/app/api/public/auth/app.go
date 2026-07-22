package auth

import (
	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/user"
)

type Implementation struct {
	authSrv  *auth.Service
	tokenSrv *token.Service
	userSrv  *user.Service
}

func New(
	authSrv *auth.Service,
	tokenSrv *token.Service,
	userSrv *user.Service,
) *Implementation {
	return &Implementation{
		authSrv:  authSrv,
		tokenSrv: tokenSrv,
		userSrv:  userSrv,
	}
}
