package auth

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// builtInOffered reports whether a built-in method may be used right now.
//
// Every failure REFUSES, matching the sign-in gates rather than the listing on
// the same endpoint. The listing degrades because it only decides which buttons
// to draw; this decides whether a code is issued, and a gate that failed open
// would hand out credentials for a method an admin turned off every time the
// database hiccups.
//
// A nil source refuses on the same reasoning: bootstrap wires the settings
// service into every binary serving this route, so a nil is a dropped wiring
// line rather than a configuration.
//
// The reason is LOGGED HERE rather than returned. This endpoint answers every
// outcome identically -- that is its whole design, so a caller could not act on
// the reason if it had one -- which makes this line the only place a disabled
// method and an unanswerable table stay distinguishable.
func (i *Implementation) builtInOffered(
	ctx context.Context,
	method entity.AuthMethodName,
	clientIP string,
) bool {
	if i.authSettings == nil {
		xlog.Error(ctx, "otp code not issued: no auth method flag source is wired",
			xfield.String("method", string(method)),
			xfield.String("client_ip", clientIP),
		)

		return false
	}

	enabled, err := i.authSettings.Enabled(ctx, method)
	if err != nil {
		xlog.Error(ctx, "otp code not issued: method flag is unreadable",
			xfield.String("method", string(method)),
			xfield.String("client_ip", clientIP),
			xfield.Error(err),
		)

		return false
	}

	if !enabled {
		xlog.Warn(ctx, "otp code not issued: method is disabled",
			xfield.String("method", string(method)),
			xfield.String("client_ip", clientIP),
		)
	}

	return enabled
}
