// Package authsettingsapi serves the admin endpoints for the built-in sign-in
// methods: read the flags, set one.
package authsettingsapi

import (
	"context"

	"github.com/ruko1202/maintmode/internal/entity"
)

// AuthSettings is the service half this API needs.
//
// Consumer-side, two methods wide: the endpoints read the flags and set one,
// and nothing here should reach the guard or the audit publisher that the
// service also owns.
type AuthSettings interface {
	List(ctx context.Context) ([]*entity.AuthMethodSetting, error)
	SetEnabled(ctx context.Context, cmd *entity.SetAuthMethodEnabledCmd) (*entity.AuthMethodSetting, error)
}

type Implementation struct {
	settingsSrv AuthSettings
}

func New(settingsSrv AuthSettings) *Implementation {
	return &Implementation{settingsSrv: settingsSrv}
}
