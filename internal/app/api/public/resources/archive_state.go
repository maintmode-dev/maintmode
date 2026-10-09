package resourcesapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// changeArchiveState is the shared body of the archive / unarchive pair: parse
// the path id, take the acting user from the context (the service records it
// in the audit trail), apply change, answer 204.
func changeArchiveState(
	c *echo.Context,
	spanName, op string,
	change func(ctx context.Context, actor *entity.User, id uuid.UUID) error,
) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), spanName)
	defer span.End()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse id failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("actor not found")
		xlog.Error(ctx, "actor user not found in echo context", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	if err := change(ctx, actor, id); err != nil {
		xlog.Error(ctx, op+" failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}
