package middlewares

import (
	"context"
	"fmt"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// headerTestRoles is the dev-only header that requests extra roles for the
// logging-in user. Deliberately absent from the public Swagger spec: it is a
// test facility, not part of the API contract.
const headerTestRoles = "X-Test-Roles"

// InjectTestRoles parses the X-Test-Roles header and stores the requested roles
// in the echo context for the login handler to fold into its command. It is a
// TEST facility: register it only under the dev environment gate in
// BaseAPIMiddlewares, so in production it is never in the chain and the header
// is physically unread. An empty/absent header is a no-op; an unknown role
// fails loudly with ErrInvalidRole (HTTP 400).
func InjectTestRoles() echo.MiddlewareFunc {
	op := "inject test roles"

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			roles, err := parseTestRoles(c.Request().Context(), c.Request().Header.Get(headerTestRoles))
			if err != nil {
				return httperrors.ToAPIError(c, op, err)
			}
			if len(roles) > 0 {
				xecho.TestRolesToEchoCtx(c, roles)
			}
			return next(c)
		}
	}
}

// parseTestRoles reads a comma-separated role list: spaces trimmed, matched
// case-insensitively, duplicates collapsed. An empty value yields no roles; an
// unknown role is a loud ErrInvalidRole rather than a silent drop.
func parseTestRoles(ctx context.Context, raw string) ([]entity.Role, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	seen := make(map[entity.Role]struct{}, len(parts))
	roles := make([]entity.Role, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		role := entity.Role(strings.ToLower(part))
		if !role.Valid(ctx) {
			return nil, fmt.Errorf("%w: %q", apperr.ErrInvalidRole, part)
		}

		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}

	return roles, nil
}
