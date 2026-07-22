package xecho

import (
	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/entity"
)

const (
	echoCtxUserKey      string = "echo_ctx_user"
	echoCtxTestRolesKey string = "echo_ctx_test_roles"
)

func valueFromEchoCtx[T any](c *echo.Context, key string) (T, bool) {
	var zero T
	v := c.Get(key)
	if v == nil {
		return zero, false
	}

	value, ok := v.(T)
	return value, ok
}

func UserToEchoCtx(c *echo.Context, user *entity.User) {
	c.Set(echoCtxUserKey, user)
}

func UserFromEchoCtx(c *echo.Context) (*entity.User, bool) {
	return valueFromEchoCtx[*entity.User](c, echoCtxUserKey)
}

// TestRolesToEchoCtx stores the dev-only X-Test-Roles the middleware parsed for
// this request. Set only by the dev-gated middleware; absent in production.
func TestRolesToEchoCtx(c *echo.Context, roles []entity.Role) {
	c.Set(echoCtxTestRolesKey, roles)
}

// TestRolesFromEchoCtx returns the roles the X-Test-Roles middleware stored for
// this request. ok is false when the middleware did not run (production, or no
// header), in which case callers treat the roles as empty.
func TestRolesFromEchoCtx(c *echo.Context) ([]entity.Role, bool) {
	return valueFromEchoCtx[[]entity.Role](c, echoCtxTestRolesKey)
}
