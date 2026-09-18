package auth

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
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
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.ListAuthMethods")
	defer span.End()

	methods := i.availableAuthMethods(ctx)

	// An empty list is the clearest signal an instance has locked itself out:
	// the login page renders no way in. It would otherwise pass through the
	// server silently, so it is logged once here -- Warn, because the usual
	// cause is a configuration an admin chose rather than a fault.
	if len(methods) == 0 {
		xlog.Warn(ctx, "no sign-in methods are available: the login page has no way in")
	}

	return c.JSON(http.StatusOK, apiauthmodels.AuthMethodsResponse{Methods: methods})
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
func (i *Implementation) availableAuthMethods(ctx context.Context) []apiauthmodels.AuthMethod {
	// The provider half comes from the live snapshot rather than a config copy
	// pinned at construction: an operator adding a provider through the registry
	// must see its button without a restart, which is the whole point of the
	// work this endpoint's comment anticipated.
	//
	// No snapshot means no providers, not a panic: the built-in methods below
	// are what makes a sign-in page usable at all, and an instance that cannot
	// list its OIDC buttons must still offer the password form rather than
	// answering 500.
	var listing []entity.LoginMethodView
	if i.authMethods != nil {
		listing = i.authMethods.Listing()
	}

	methods := make([]apiauthmodels.AuthMethod, 0, 2+len(listing))

	// One read for both built-ins rather than one each. At two rows the
	// difference is nothing, but this is the login page's first request from
	// every visitor and the only affected endpoint that does no hashing and no
	// IdP call, so the per-method shape is the one worth not establishing.
	offered := i.offeredBuiltIns(ctx)

	if offered[entity.AuthMethodNameEmailPassword] {
		methods = append(methods, apiauthmodels.AuthMethod{
			ID:          "email_password",
			Type:        apiauthmodels.AuthMethodTypePassword,
			DisplayName: "Password",
		})
	}

	if offered[entity.AuthMethodNameEmailOTP] {
		methods = append(methods, apiauthmodels.AuthMethod{
			ID:          "email_otp",
			Type:        apiauthmodels.AuthMethodTypeCode,
			DisplayName: "Email code",
		})
	}

	// The snapshot sorts its listing, which matters twice: map iteration is
	// randomized, so an unsorted list would reshuffle the buttons between
	// requests and between replicas, and this endpoint's contract is that two
	// callers get identical bytes.
	//
	// Only the name and the label go out. An instance that is configured but
	// whose discovery has not resolved still appears: this reports what is
	// configured, not what is reachable, and probing every IdP to render a login
	// page would be a self-inflicted outage.
	for _, view := range listing {
		methods = append(methods, apiauthmodels.AuthMethod{
			ID:          string(view.ID),
			Type:        apiauthmodels.AuthMethodTypeRedirect,
			DisplayName: view.DisplayName,
		})
	}

	return methods
}

// offeredBuiltIns answers, in one read, which built-ins the login page should
// show.
//
// The listing DEGRADES rather than failing: a method whose flag cannot be read
// drops out and the rest of the response still renders. That is deliberately
// the opposite of what the sign-in gate does with the same error, and the
// asymmetry is the point -- this endpoint grants nothing, so hiding a button is
// the cheap direction to be wrong in, while answering 500 would leave the login
// page unable to render at all. A caller who then guesses a hidden method still
// meets the gate, which failed closed, so the listing may understate what works
// and can never overstate it.
//
// A binary wired without the flags shows NOTHING rather than everything: the
// gate refuses those methods too, so listing them would advertise credentials
// that will not work.
func (i *Implementation) offeredBuiltIns(ctx context.Context) map[entity.AuthMethodName]bool {
	offered := make(map[entity.AuthMethodName]bool, 2)

	if i.authSettings == nil {
		return offered
	}

	settings, err := i.authSettings.List(ctx)
	if err != nil {
		// Error, not Warn: per the severity split this is the database failing
		// to answer, not an instance configured this way, and it is refusing
		// sign-ins that should have succeeded.
		xlog.Error(ctx, "auth method flags unreadable, omitting every built-in from the listing",
			xfield.Error(err),
		)

		return offered
	}

	for _, s := range settings {
		offered[s.Method] = s.Enabled
	}

	return offered
}
