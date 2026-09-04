package auth

import (
	"context"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// maxEmailLen bounds the body's email. RFC 5321 caps a real address at 254
// characters; the limit exists to stop an unauthenticated caller writing an
// arbitrarily large string into the audit trail, not to be exact about the RFC.
const maxEmailLen = 254

// LoginWithPassword godoc
// @Summary Sign in with a password
// @Description Issues a backend token pair for a password-based sign-in. Every failure — wrong password, blocked account, refused signup, malformed body — answers with the same 401, so the response never reveals whether an account exists or what state it is in.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.LoginWithPasswordRequest true "Credentials"
// @Success 200 {object} apiauthmodels.TokenPairResponse
// @Failure 401 {object} httperrors.ErrorResponse "Authentication failed"
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Router /api/v1/login/password [post]
func (i *Implementation) LoginWithPassword(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.LoginWithPassword")
	defer span.End()

	body := new(apiauthmodels.LoginWithPasswordRequest)
	if err := c.Bind(body); err != nil {
		// Answered like every other failure rather than as a 400. A malformed
		// body reveals nothing about an account by itself, but "every failure of
		// this endpoint looks the same" is a rule that is only worth having if it
		// has no exceptions to reason about — and a caller who can tell a
		// bind failure from a rejected password has one bit more than intended.
		return unauthorized(ctx, c, "malformed request body", err)
	}

	cmd := &entity.LoginWithPasswordCmd{
		Email:      body.Email,
		Password:   body.Password,
		RememberMe: body.RememberMe,
		ClientIP:   c.RealIP(),
		UserAgent:  c.Request().UserAgent(),
	}

	if err := validateLoginWithPasswordCmd(ctx, cmd); err != nil {
		// A validation failure is answered like every other failure: an empty
		// password is a failed sign-in attempt, and telling the caller their
		// input was malformed rather than wrong is a distinction worth nothing
		// to a legitimate operator and something to an attacker.
		return unauthorized(ctx, c, "invalid request", err)
	}

	pair, err := i.authSrv.LoginWithPassword(ctx, cmd)
	if err != nil {
		// The reason is not lost: it goes to the audit trail, and unauthorized
		// logs it below.
		return unauthorized(ctx, c, "authentication failed", err)
	}

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, apiauthmodels.ToAPITokenPairResponse(pair))
}

// unauthorized is the single response every failed password login produces —
// bind failure, validation failure, and every error the service can return.
//
// Deliberately not the shared mapper, which answers each of those differently:
// a blocked user gets 401 carrying the wrapped error text, a refused signup 403
// signup_disabled, an exhausted seat cap 403 with the exact seat counts, an
// unregistered method 400 naming it. Every one of those tells an
// unauthenticated caller something about the account or the deployment. It is
// local rather than a change to the shared mapper because ErrUserBlocked's
// global mapping is also what refresh and logout rely on.
//
// One function, one call shape: the guarantee is that no caller can tell two
// failures apart, and that is easiest to keep true when there is exactly one
// place the response is built. The collapse is unconditional by construction —
// there is no error parameter feeding the response, so there is nothing to get
// wrong per call site.
func unauthorized(ctx context.Context, c *echo.Context, reason string, err error) error {
	// Logged here rather than at each call site so that every failure is on
	// record exactly once, with its cause. The response says nothing, so this
	// line is the only thing standing between an operator and an undiagnosable
	// lockout — reading the audit log needs the admin session they are trying to
	// recover.
	//
	// WARN, not ERROR: a rejected sign-in is an expected event on a
	// permanently-live endpoint, and logging it at ERROR would bury the failures
	// that are genuinely the service's fault. A sustained rate of these is what
	// the BootstrapLoginFailing alert is for.
	xlog.Warn(ctx, "password login rejected",
		xfield.String("reason", reason),
		xfield.Error(err),
	)

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusUnauthorized,
		httperrors.NewErrorResponse(httperrors.ErrUnauthorized, "authentication failed"))
}

// validateLoginWithPasswordCmd rejects a request before it reaches the service.
//
// Email is validated even though bootstrap ignores it for identity. It is not
// inert: on a credential mismatch it is what the audit record is attributed to
// (there are no claims to attribute to at that point), so an unvalidated field
// on an unauthenticated endpoint becomes an unbounded attacker-chosen string
// written to audit_log — whose `actor` column carries a btree index that errors
// above ~2704 bytes. Bounding it here keeps a brute-force attempt from
// degrading the audit trail, which is one of the controls that makes a
// permanently-live break-glass endpoint acceptable in the first place.
//
// A validation failure answers with the same 401 as every other failure, so
// this adds no way to tell requests apart.
func validateLoginWithPasswordCmd(ctx context.Context, cmd *entity.LoginWithPasswordCmd) error {
	return validation.ValidateStructWithContext(ctx, cmd,
		validation.Field(&cmd.Email, validation.Required, validation.Length(0, maxEmailLen), is.EmailFormat),
		validation.Field(&cmd.Password, validation.Required),
		validation.Field(&cmd.ClientIP, validation.Required),
	)
}
