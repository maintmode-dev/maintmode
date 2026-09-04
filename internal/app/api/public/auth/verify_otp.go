package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xemail"
)

const (
	// otpCodeLen is the length of an emailed code. Bounded here so an
	// unauthenticated caller cannot push an arbitrarily long string further in.
	otpCodeLen = 6
	// maxSessionNonceLen bounds the nonce. A real one is 44 base64url characters;
	// the cap is generous rather than exact, since its job is to stop unbounded
	// input rather than to re-validate the generator's output.
	maxSessionNonceLen = 64
)

// VerifyOTP godoc
// @Summary Sign in with a one-time code
// @Description Redeems a code emailed by /login/otp/request and issues a token pair. Requires both the code and the session_nonce returned when it was requested; a mismatch answers otp_session_mismatch, telling the caller to request a new code. Every other failure — wrong code, expired code, exhausted attempts, unknown address, malformed body — answers with the same 401, so the response never reveals whether an account exists.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.VerifyOTPRequest true "Code and session nonce"
// @Success 200 {object} apiauthmodels.TokenPairResponse
// @Failure 401 {object} httperrors.ErrorResponse "Authentication failed, or otp_session_mismatch"
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Router /api/v1/login/otp/verify [post]
func (i *Implementation) VerifyOTP(c *echo.Context) error {
	start := time.Now()

	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.VerifyOTP")
	defer span.End()

	body := new(apiauthmodels.VerifyOTPRequest)
	if err := c.Bind(body); err != nil {
		// A body that did not parse names no address, so there is nothing to
		// attribute an audit record to — see otpRejected.
		return i.otpRejected(ctx, c, start, "malformed request body", err)
	}

	cmd := &entity.VerifyOTPCmd{
		// Normalized once, here, through the same function the rate limiter
		// keys on. Two consumers disagreeing about what "the same address" means
		// is a security bug: the limiter would bucket one string while the
		// lookup resolved another, splitting a victim's budget.
		Email:        xemail.Normalize(body.Email),
		Code:         body.Code,
		SessionNonce: body.SessionNonce,
		RememberMe:   body.RememberMe,
		ClientIP:     c.RealIP(),
		UserAgent:    c.Request().UserAgent(),
	}

	if err := validateVerifyOTPCmd(ctx, cmd); err != nil {
		// Answered like every other failure: an empty or malformed field is a
		// failed sign-in attempt, and telling the caller their input was
		// malformed rather than wrong is worth nothing to an operator and
		// something to an attacker.
		//
		// NOT audited, and the reason is the length check that just failed. A
		// validation failure includes "the address is longer than 254
		// characters", so auditing here would write an attacker-chosen string of
		// any size into audit_log.actor and entity_id — both indexed, both
		// erroring above ~2704 bytes. The audited paths are the ones downstream
		// of this check, where the bound holds.
		return i.otpRejected(ctx, c, start, "invalid request", err)
	}

	pair, err := i.authSrv.LoginWithOTP(ctx, cmd)
	if err != nil {
		if errors.Is(err, apperr.ErrOTPSessionMismatch) {
			return i.otpSessionMismatch(ctx, c, start, err)
		}

		return i.otpRejected(ctx, c, start, "authentication failed", err)
	}

	return i.succeeded(c, start, pair)
}

// succeeded writes the one success this endpoint produces, and waits out the
// same floor every failure does.
//
// Flooring a success looks redundant -- the body already announces it -- and it
// is not. The endpoint's guarantee is that latency carries no information, and
// leaving one return path unfloored breaks that in a specific way: an attacker
// holding a victim's code but not their nonce gets otp_session_mismatch at the
// floor, while the correct pair answers as fast as issuance allows. That is a
// timing signal for "the code is right, only the binding is wrong", which is
// exactly the state a code relayed to a third party is in.
//
// It also keeps the single-exit discipline the failure builders rely on: every
// response this handler writes goes through a function that floors it, so there
// is no path a later edit can add that quietly skips the wait.
func (i *Implementation) succeeded(c *echo.Context, start time.Time, pair *entity.TokenPair) error {
	i.waitOutFloor(start)

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")

	return c.JSON(http.StatusOK, apiauthmodels.ToAPITokenPairResponse(pair))
}

// otpRejected is the single response every failed redemption produces, apart
// from the nonce mismatch below.
//
// It waits out the same response floor the request endpoint uses, and the reason
// is an oracle that the uniform status alone does not close. An address with no
// account returns after ONE indexed SELECT; an address with one costs a second
// SELECT, an attempt-claim UPDATE, a hash and a comparison -- a read path against
// a read-write path, a difference that grows under load rather than shrinking.
// The rate limiters do not help here: enumeration needs one request per
// candidate address, not many.
//
// One function, one call shape: the guarantee is that no caller can tell two
// failures apart, and that is easiest to keep true when there is exactly one
// place the response is built. The collapse is unconditional by construction —
// no error feeds the response, so there is nothing to get wrong per call site.
//
// The cause is logged rather than returned, at WARN: a rejected sign-in is
// expected traffic on a permanently-live public endpoint, and logging it at
// ERROR would bury the failures that are genuinely the service's fault.
func (i *Implementation) otpRejected(
	ctx context.Context,
	c *echo.Context,
	start time.Time,
	reason string,
	err error,
) error {
	xlog.Warn(ctx, "otp verification rejected",
		xfield.String("reason", reason),
		xfield.Error(err),
	)

	i.waitOutFloor(start)

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")

	return c.JSON(http.StatusUnauthorized,
		httperrors.NewErrorResponse(httperrors.ErrUnauthorized, "authentication failed"))
}

// otpSessionMismatch answers the one failure that is deliberately distinguishable.
//
// Provoking it requires already holding a live code, so it discloses nothing
// about whether an account exists. What it buys is a user who closed the tab
// while the mail was in flight being told to ask for a new code, rather than
// retyping a correct code against a nonce that no longer exists. The message
// says what to do and nothing else — not whose nonce, not what was expected, not
// that a code exists for the address.
func (i *Implementation) otpSessionMismatch(
	ctx context.Context,
	c *echo.Context,
	start time.Time,
	err error,
) error {
	xlog.Warn(ctx, "otp verification rejected",
		xfield.String("reason", "session nonce mismatch"),
		xfield.Error(err),
	)

	// Floored like every other failure. Reaching this branch already implies a
	// live code, so it is not itself an existence oracle -- but leaving it
	// unfloored would make it the one failure with a distinctive latency, which
	// is a distinction of exactly the kind the rest of this endpoint removes.
	i.waitOutFloor(start)

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")

	return c.JSON(http.StatusUnauthorized,
		httperrors.NewErrorResponse(httperrors.ErrOTPSessionMismatch, "request a new code"))
}

// canonicalEmail rejects an address that normalization would have changed.
//
// The command already carries the normalized value, so this fires only when the
// caller sent something that needed normalizing -- and the interesting case is
// not a stray space but an invisible one. is.EmailFormat accepts a trailing
// U+200B, U+FEFF or U+2060, and each variant is a different string: audited as
// written, and (before the shared normalizer) a different limiter bucket. An
// address carrying an invisible character is not a typo a user makes, so it is
// refused rather than quietly repaired -- repairing it would still let a caller
// write unbounded distinct values into the indexed audit columns.
func canonicalEmail(value any) error {
	email, ok := value.(string)
	if !ok || xemail.IsCanonical(email) {
		return nil
	}

	return errNonCanonicalEmail
}

// errNonCanonicalEmail is answered like every other validation failure: the
// caller gets the same 401 as a wrong code.
var errNonCanonicalEmail = errors.New("email is not canonical")

// validateVerifyOTPCmd bounds every attacker-supplied field before it reaches
// the service.
//
// The bounds are not cosmetic on an unauthenticated endpoint: the address flows
// into audit_log.actor and entity_id, both carrying btree indexes that error
// above ~2704 bytes, and the code and nonce flow into log lines. The exact code
// length also keeps the constant-time comparison from being handed a value whose
// size alone is informative.
func validateVerifyOTPCmd(ctx context.Context, cmd *entity.VerifyOTPCmd) error {
	return validation.ValidateStructWithContext(ctx, cmd,
		validation.Field(&cmd.Email, validation.Required, validation.Length(0, maxEmailLen), is.EmailFormat,
			validation.By(canonicalEmail)),
		validation.Field(&cmd.Code, validation.Required, validation.Length(otpCodeLen, otpCodeLen), is.Digit),
		validation.Field(&cmd.SessionNonce, validation.Required, validation.Length(0, maxSessionNonceLen)),
		validation.Field(&cmd.ClientIP, validation.Required),
	)
}
