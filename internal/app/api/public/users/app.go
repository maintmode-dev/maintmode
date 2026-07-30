package users

import (
	"github.com/ruko1202/maintmode/internal/services/license"
	"github.com/ruko1202/maintmode/internal/services/user"
)

type Implementation struct {
	userSrv *user.Service
	// licenseSrv is held as the interface, not the concrete service: on a
	// self-hosted instance the concrete one does not exist and Noop answers.
	licenseSrv license.Enforcement
}

func New(
	userSrv *user.Service,
	licenseSrv license.Enforcement,
) *Implementation {
	return &Implementation{
		userSrv:    userSrv,
		licenseSrv: licenseSrv,
	}
}
