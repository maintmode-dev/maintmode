// Package apinotifications exposes HTTP handlers for the
// notifications domain. Today it serves the channel catalog used by
// the admin UI's channel picker.
package apinotifications

import (
	"github.com/ruko1202/maintmode/internal/services/notifytargets"
)

type Implementation struct {
	notifyTargets *notifytargets.Service
}

func New(notifyTargets *notifytargets.Service) *Implementation {
	return &Implementation{
		notifyTargets: notifyTargets,
	}
}
