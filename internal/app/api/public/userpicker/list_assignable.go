package userpicker

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/userpicker/models"
	usersmodels "github.com/ruko1202/maintmode/internal/app/api/public/users/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

const (
	defaultUsersLimit  = 50
	maxUsersLimit      = 200
	defaultUsersOffset = 0
)

// ListAssignableUsers godoc
// @Summary List assignable users
// @Description Returns users eligible for maintenance assignment (reviewer/owner, notify targets), ordered by display_name ASC (id ASC tie-breaker). Blocked users are always excluded.
// @Description Malformed pagination/filter params are coerced to defaults rather than rejected.
// @Tags Maintenances
// @Produce json
// @Description has_messenger_tag is always present. It reports only WHETHER the user has any messenger handle configured, never which one or its value, and it is meaningful only for callers permitted to plan maintenances (editor and above) — for anyone below it is hard false on every row.
// @Param search query string false "Case-insensitive partial match on display_name or email. The telegram_tag and slack_tag columns are NOT matched here — this response never carries their values (only the derived has_messenger_tag boolean), and matching them would let any caller confirm which tag belongs to which person. Tag search lives on the admin user list."
// @Param roles query []string false "Keep only users having ANY of these roles (guest|editor|reviewer|admin)" collectionFormat(multi)
// @Param limit query int false "Page size (max 200)" default(50)
// @Param offset query int false "Pagination offset" default(0)
// @Success 200 {object} apimodels.ListAssignableUsersResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid role filter"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Security BearerAuth
// @Router /api/v1/users/assignable [get]
func (i *Implementation) ListAssignableUsers(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.UserPicker.ListAssignableUsers")
	defer span.End()
	op := "list assignable users"

	query, err := queryToListAssignableQuery(c)
	if err != nil {
		xlog.Error(ctx, "invalid list assignable users query", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	result, err := i.pickerSrv.ListAssignable(ctx, query)
	if err != nil {
		xlog.Error(ctx, "list assignable users failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, &apimodels.ListAssignableUsersResponse{
		Users:  apimodels.ToAPIAssignableUsers(result.Users),
		Total:  result.Total,
		Limit:  result.Limit,
		Offset: result.Offset,
	})
}

// queryToListAssignableQuery parses pagination/filter params. Malformed or
// out-of-range pagination is coerced to safe defaults (read-only picker); an
// unknown role filter is an explicit client mistake and is rejected.
func queryToListAssignableQuery(c *echo.Context) (*entity.ListAssignableUsersQuery, error) {
	limit, err := echo.QueryParamOr[int64](c, "limit", defaultUsersLimit)
	if err != nil || limit <= 0 || limit > maxUsersLimit {
		limit = defaultUsersLimit
	}

	offset, err := echo.QueryParamOr[int64](c, "offset", defaultUsersOffset)
	if err != nil || offset < 0 {
		offset = defaultUsersOffset
	}

	roles, err := usersmodels.FromAPIRolesFilter(c.QueryParams()["roles"])
	if err != nil {
		return nil, err
	}

	return &entity.ListAssignableUsersQuery{
		Search: c.QueryParam("search"),
		Roles:  roles,
		Limit:  limit,
		Offset: offset,
	}, nil
}
