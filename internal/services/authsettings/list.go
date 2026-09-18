package authsettings

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// List returns every built-in method flag.
func (s *Service) List(ctx context.Context) ([]*entity.AuthMethodSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AuthSettings.List")
	defer span.End()

	return s.store.List(ctx)
}

// Enabled reports whether one method may be used right now.
//
// This is the read the sign-in gates and the public listing go through, so its
// error behavior is the feature's availability behavior: a fault is returned,
// never swallowed into a default. Guessing "enabled" would reopen a path an
// admin closed the moment the database hiccups; guessing "disabled" would lock
// the instance out for the same reason. The callers decide what to do with the
// error, and they do not agree -- the gates refuse, the listing degrades -- but
// neither of them gets to pretend it knows the flag.
func (s *Service) Enabled(ctx context.Context, method entity.AuthMethodName) (bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AuthSettings.Enabled")
	defer span.End()

	found, err := s.store.GetByMethod(ctx, method)
	if err != nil {
		return false, err
	}

	return found.Enabled, nil
}
