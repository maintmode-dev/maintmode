package xecho

import (
	"github.com/labstack/echo/v4"

	"github.com/ruko1202/maintmode/internal/entity"
)

const (
	echoCtxUserKey string = "echo_ctx_user"
)

func valueFromEchoCtx[T any](c echo.Context, key string) (T, bool) {
	var zero T
	v := c.Get(key)
	if v == nil {
		return zero, false
	}

	value, ok := v.(T)
	return value, ok
}

func UserToEchoCtx(c echo.Context, user *entity.User) {
	c.Set(echoCtxUserKey, user)
}

func UserFromEchoCtx(c echo.Context) (*entity.User, bool) {
	return valueFromEchoCtx[*entity.User](c, echoCtxUserKey)
}
