package auth

import (
	"github.com/ruko1202/maintmode/internal/services/auth"
	statecodec "github.com/ruko1202/maintmode/internal/services/state_codec"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/user"
)

type Implementation struct {
	authSrv     *auth.Service
	tokenSrv    *token.Service
	userSrv     *user.Service
	stateCodec  *statecodec.Service
	frontendURL string
}

func New(
	authSrv *auth.Service,
	tokenSrv *token.Service,
	userSrv *user.Service,
	stateCodec *statecodec.Service,
	frontendURL string,
) *Implementation {
	return &Implementation{
		authSrv:     authSrv,
		tokenSrv:    tokenSrv,
		userSrv:     userSrv,
		stateCodec:  stateCodec,
		frontendURL: frontendURL,
	}
}
