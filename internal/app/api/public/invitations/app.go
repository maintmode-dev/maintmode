package invitations

import (
	"github.com/ruko1202/maintmode/internal/services/invitation"
)

type Implementation struct {
	invSrv *invitation.Service
}

func New(invSrv *invitation.Service) *Implementation {
	return &Implementation{
		invSrv: invSrv,
	}
}
