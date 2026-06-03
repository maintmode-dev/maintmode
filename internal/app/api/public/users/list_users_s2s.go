package users

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/users/models"
)

// ListUsersS2S godoc
// @Summary List users (service-to-service)
// @Description Service-to-service paginated user listing, ordered by display_name ASC (id ASC tie-breaker). Authenticated by the S2S token, not a user scope. Returns a neutral user shape (id, email, display_name, roles) for consuming services such as maintenance assignment pickers. Pass active=true to hide blocked users.
// @Description Malformed pagination/filter params are coerced to defaults rather than rejected.
// @Tags Users
// @Produce json
// @Param search query string false "Case-insensitive partial match on display_name or email"
// @Param role query string false "Keep only users having this role (guest|editor|reviewer|admin)"
// @Param active query bool false "When true, hide blocked users" default(false)
// @Param limit query int false "Page size (max 200)" default(50)
// @Param offset query int false "Pagination offset" default(0)
// @Success 200 {object} apimodels.ListS2SUsersResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid role filter"
// @Failure 401 {object} httperrors.ErrorResponse "Missing or invalid S2S token"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/s2s/users [get]
func (i *Implementation) ListUsersS2S(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Users.ListUsersS2S")
	defer span.End()
	op := "list users (s2s)"

	cmd, err := queryToListUsersCmd(c)
	if err != nil {
		xlog.Error(ctx, "invalid s2s list users query", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	result, err := i.userSrv.ListUsers(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "s2s list users failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, &apimodels.ListS2SUsersResponse{
		Users:  apimodels.ToS2SUsers(result.Users),
		Total:  result.Total,
		Limit:  cmd.Limit,
		Offset: cmd.Offset,
	})
}
