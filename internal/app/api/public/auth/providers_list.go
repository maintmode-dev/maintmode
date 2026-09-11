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

	return c.JSON(http.StatusOK, apiauthmodels.AuthMethodsResponse{Methods: i.availableAuthMethods()})
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
// The OIDC entries are the exception to "unconditional": one per configured
// instance, which is how an operator adds a corporate SSO button without a
// release. Both built-in methods below are unconditional, and email_password is
// the one worth
// explaining. It is NOT gated on a configured bootstrap password, and the reason
// changed shape without changing the answer. An instance with no break-glass used
// to be impossible; it is now an ordinary configuration. So the gate is no longer
// refused as a branch that never runs -- it is refused because it would LEAK.
//
// This endpoint is public and unauthenticated. A predicate over the bootstrap
// password would answer "does this deployment have an emergency entrance?" in one
// GET, with no sign-in attempt, no rate limiter and no audit record -- strictly
// worse than the timing channel the decoy hash on the sign-in path exists to
// close. Do not add one, however stale this comment may look.
//
// It also matches the intent: from outside, break-glass must be indistinguishable
// from an ordinary password sign-in, and an instance must never be able to hide
// the form that recovers it.
//
// bootstrap is not its own element for the same reason. It and email_password
// lead to the same form on the same endpoint, so two elements would draw two
// identical forms.
// The ids and labels stay literals rather than constants, and goconst is
// silenced rather than obeyed: the repeats it counts are in the contract tests,
// which assert these values as literals on purpose. A test comparing a constant
// against the same constant would pass through any rename and prove nothing —
// two independent spellings of the wire format is exactly the point.
//
//nolint:goconst
func (i *Implementation) availableAuthMethods() []apiauthmodels.AuthMethod {
	instances := i.oidcProviders.InstanceNames()

	methods := make([]apiauthmodels.AuthMethod, 0, 2+len(instances))
	methods = append(methods,
		apiauthmodels.AuthMethod{
			ID:          "email_password",
			Type:        apiauthmodels.AuthMethodTypePassword,
			DisplayName: "Password",
		},
		apiauthmodels.AuthMethod{
			ID:          "email_otp",
			Type:        apiauthmodels.AuthMethodTypeCode,
			DisplayName: "Email code",
		},
	)

	// InstanceNames sorts, which matters twice: map iteration is randomized, so
	// an unsorted list would reshuffle the buttons between requests and between
	// replicas, and this endpoint's contract is that two callers get identical
	// bytes.
	//
	// Only the name and the label go out. An instance that is configured but
	// whose discovery has not resolved still appears: this reports what is
	// configured, not what is reachable, and probing every IdP to render a login
	// page would be a self-inflicted outage.
	for _, name := range instances {
		methods = append(methods, apiauthmodels.AuthMethod{
			ID:          name,
			Type:        apiauthmodels.AuthMethodTypeRedirect,
			DisplayName: i.oidcProviders.OIDC[name].DisplayName,
		})
	}

	return methods
}
