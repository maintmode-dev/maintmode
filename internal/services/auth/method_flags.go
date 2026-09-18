package auth

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// methodOffered reports whether a built-in method may sign anyone in right now.
//
// Every failure REFUSES, which is the opposite of what the public listing does
// with the same error: that endpoint grants nothing, so it degrades to showing
// fewer buttons, while a gate that failed open would reopen a sign-in path an
// admin closed every time the database hiccups. A security control that
// switches off under load is not a control.
//
// What makes failing closed survivable is that break-glass does not read this
// table: the password gate skips the stored-credential step and falls through
// to it, so an outage refuses ordinary sign-ins and leaves the emergency
// entrance open.
//
// A nil source refuses for the same reason. Bootstrap wires the settings
// service into every binary that serves these paths, so a nil is a dropped
// wiring line rather than a configuration -- and a dropped line must not
// silently re-open every method an admin closed.
//
// The reason is LOGGED HERE rather than returned, because here is the only
// place it is known and the only thing anyone does with it. Callers cannot act
// on it: the response is uniform by design, and so is the audit record (see
// loginWithSeed), so an operator correlating "sign-ins started failing at
// 14:32" against a change has this line and nothing else.
func (s *Service) methodOffered(
	ctx context.Context,
	method entity.AuthMethodName,
	clientIP string,
) bool {
	if s.methodFlags == nil {
		xlog.Error(ctx, "sign-in refused: no auth method flag source is wired",
			xfield.String("method", string(method)),
			xfield.String("client_ip", clientIP),
		)

		return false
	}

	enabled, err := s.methodFlags.Enabled(ctx, method)
	if err != nil {
		xlog.Error(ctx, "sign-in refused: method flag is unreadable",
			xfield.String("method", string(method)),
			xfield.String("client_ip", clientIP),
			xfield.Error(err),
		)

		return false
	}

	if !enabled {
		xlog.Warn(ctx, "sign-in refused: method is disabled",
			xfield.String("method", string(method)),
			xfield.String("client_ip", clientIP),
		)
	}

	return enabled
}
