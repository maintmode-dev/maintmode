package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
)

// ListAuthMethods godoc
// @Summary List the ways to sign in
// @Description Returns the sign-in methods this instance offers, so the login page knows what to draw. Public and unauthenticated: the response is identical for every caller and reveals nothing about any account or provider configuration.
// @Tags Auth
// @Produce json
// @Success 200 {object} apiauthmodels.AuthMethodsResponse
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Router /api/v1/auth/providers [get]
func (i *Implementation) ListAuthMethods(c *echo.Context) error {
	_, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.ListAuthMethods")
	defer span.End()

	return c.JSON(http.StatusOK, apiauthmodels.AuthMethodsResponse{Methods: availableAuthMethods()})
}

// availableAuthMethods assembles the list.
//
// It is one function rather than a literal inside the handler because the SOURCE
// of this list is going to move: a later change reads it from a table an admin
// can toggle, and the ticket for it states the response shape will not change.
// Keeping the assembly in one place makes that a replacement of this body and
// nothing else. It is deliberately not an interface or a registry — there is one
// caller and one implementation, and building an extension point now would be
// guessing at a design that change has not made yet.
//
// Both methods are unconditional today, and email_password is the one worth
// explaining. It is NOT gated on a configured bootstrap password, because there
// is no state in which one is absent: an empty password means "generate one at
// startup" and validateBootstrapConfig makes the address mandatory, failing boot
// outright when it is missing. A predicate over that would be a branch that never
// runs, pinned by a test asserting a state production cannot reach. It also
// matches the intent: from outside, break-glass must be indistinguishable from an
// ordinary password sign-in, and an instance must never be able to hide the form
// that recovers it.
//
// bootstrap is not its own element for the same reason. It and email_password
// lead to the same form on the same endpoint, so two elements would draw two
// identical forms.
func availableAuthMethods() []apiauthmodels.AuthMethod {
	return []apiauthmodels.AuthMethod{
		{
			ID:          "email_password",
			Type:        apiauthmodels.AuthMethodTypePassword,
			DisplayName: "Password",
		},
		{
			ID:          "email_otp",
			Type:        apiauthmodels.AuthMethodTypeCode,
			DisplayName: "Email code",
		},
	}
}
