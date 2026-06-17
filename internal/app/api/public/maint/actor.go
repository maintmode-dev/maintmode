package apimaint

import (
	"fmt"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// actorFromCtx resolves the authenticated user that the auth middleware put on
// the echo context. It is the audit actor (RUK-182): every mutation records who
// performed it, so a missing actor is a hard failure, not a silent system
// fallback (the project does not degrade a required actor). A non-nil returned
// error is the already-shaped API error response — the caller returns it as-is.
func (i *Implementation) actorFromCtx(c *echo.Context, op string) (*entity.User, error) {
	ctx := c.Request().Context()

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("actor not found")
		xlog.Error(ctx, "actor user not found in echo context", xfield.Error(err))
		return nil, httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	return actor, nil
}
