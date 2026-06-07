// Package apinotifications exposes HTTP handlers for the
// notifications domain. Today it serves the channel catalog used by
// the admin UI's channel picker.
package apinotifications

import (
	"github.com/ruko1202/maintmode/internal/services/notifytargets"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
)

type Implementation struct {
	notifyTargets  *notifytargets.Service
	userSummarySrv *usersummary.Service
}

func New(notifyTargets *notifytargets.Service, userSummarySrv *usersummary.Service) *Implementation {
	return &Implementation{
		notifyTargets:  notifyTargets,
		userSummarySrv: userSummarySrv,
	}
}
