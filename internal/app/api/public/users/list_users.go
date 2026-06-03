package users

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/users/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

const (
	defaultUsersLimit  = 50
	maxUsersLimit      = 200
	defaultUsersOffset = 0
)

// ListUsers godoc
// @Summary List users
// @Description Returns a paginated list of users ordered by display_name ASC (id ASC as tie-breaker). Requires admin privileges.
// @Description connected_providers lists the OAuth providers linked to each user; oauth_provider is the first linked provider ("unknown" when none). last_seen_at is null (not tracked yet).
// @Description Malformed pagination/filter params are coerced to defaults rather than rejected.
// @Tags Users
// @Produce json
// @Param search query string false "Case-insensitive partial match on display_name or email"
// @Param role query string false "Keep only users having this role (guest|editor|reviewer|admin)"
// @Param limit query int false "Page size (max 200)" default(50)
// @Param offset query int false "Pagination offset" default(0)
// @Success 200 {object} apimodels.ListUsersResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid role filter"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/users/list [get]
func (i *Implementation) ListUsers(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Users.ListUsers")
	defer span.End()
	op := "list users"

	cmd, err := queryToListUsersCmd(c)
	if err != nil {
		xlog.Error(ctx, "invalid list users query", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	result, err := i.userSrv.ListUsers(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "list users failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, &apimodels.ListUsersResponse{
		Users:  apimodels.ToAPIUsers(result.Users, result.ActiveAdminCount, result.ProvidersByUser),
		Total:  result.Total,
		Limit:  cmd.Limit,
		Offset: cmd.Offset,
	})
}

// queryToListUsersCmd parses pagination/filter query params. Malformed or
// out-of-range pagination values are silently coerced to safe defaults rather
// than rejected — this is a read-only admin reference list, so a best-effort
// response is preferable to a 400. An unknown role filter, however, is an
// explicit client mistake and is rejected with a validation error.
func queryToListUsersCmd(c *echo.Context) (*entity.ListUsersCmd, error) {
	limit, err := echo.QueryParamOr[int64](c, "limit", defaultUsersLimit)
	if err != nil || limit <= 0 || limit > maxUsersLimit {
		limit = defaultUsersLimit
	}

	offset, err := echo.QueryParamOr[int64](c, "offset", defaultUsersOffset)
	if err != nil || offset < 0 {
		offset = defaultUsersOffset
	}

	role, err := apimodels.FromAPIRoleFilter(c.QueryParam("role"))
	if err != nil {
		return nil, err
	}

	return &entity.ListUsersCmd{
		Search: c.QueryParam("search"),
		Role:   role,
		Limit:  limit,
		Offset: offset,
	}, nil
}
