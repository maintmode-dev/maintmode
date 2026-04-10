package roles

import (
	"github.com/ruko1202/maintmode/internal/services/user"
)

type Implementation struct {
	userSrv *user.Service
}

func New(
	userSrv *user.Service,
) *Implementation {
	return &Implementation{
		userSrv: userSrv,
	}
}
