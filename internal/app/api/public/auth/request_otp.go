package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/utils/xcripto"
	"github.com/ruko1202/maintmode/internal/utils/xemail"
)

const (
	// otpNonceBytes matches the entropy the service mints, so the placeholder
	// nonce returned on a failed request is the same shape as a real one.
	otpNonceBytes = 32
	// otpNonceEncodedLen is that many bytes in padded base64url: 44 characters,
	// the last of which is always "=".
	otpNonceEncodedLen = 44
)

// RequestOTP godoc
// @Summary Request a one-time sign-in code
// @Description Emails a one-time code to the address, if it belongs to an account. Answers 202 in every case — unknown address, blocked account, malformed body — so the response never reveals whether an account exists. The returned session_nonce binds the code to the client that asked for it and must be presented when the code is verified; it is never emailed.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.RequestOTPRequest true "Address to send the code to"
// @Success 202 {object} apiauthmodels.RequestOTPResponse
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Router /api/v1/login/otp/request [post]
func (i *Implementation) RequestOTP(c *echo.Context) error {
	start := time.Now()

	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.RequestOTP")
	defer span.End()

	body := new(apiauthmodels.RequestOTPRequest)
	if err := c.Bind(body); err != nil {
		// Answered like everything else rather than as a 400. A malformed body
		// reveals nothing about an account by itself, but "every response of this
		// endpoint looks the same" is only worth having if it has no exceptions
		// to reason about.
		return i.rejected(ctx, c, start, "malformed request body", err)
	}

	if err := validateRequestOTP(ctx, body); err != nil {
		return i.rejected(ctx, c, start, "invalid request", err)
	}

	// Normalized through the same function the limiter keys on, for the reason
	// spelled out on the verify endpoint: these two routes share one per-address
	// tier, so normalizing on only one of them leaves the other able to split a
	// victim's budget.
	nonce, err := i.otpSrv.Request(ctx, xemail.Normalize(body.Email))
	if err != nil {
		return i.rejected(ctx, c, start, "issue failed", err)
	}

	return i.accepted(c, start, nonce)
}

// accepted writes the one response this endpoint produces, after waiting out
// the floor.
//
// Building the body and waiting happen together, in the only place a response
// is written, so no return path can answer early. The wait cannot be moved to a
// defer in the handler either: by the time a deferred call runs, c.JSON has
// already put the body on the wire, and the timing is observable no matter how
// long the handler lingers afterwards.
//
// The floor bounds the fast branch from below; it cannot bound the slow branch
// from above, so its configured default has to stay above what issuance
// actually costs. A 429 from the rate limiter is produced by middleware before
// any of this runs, and is the one response that does not pass through here.
func (i *Implementation) accepted(c *echo.Context, start time.Time, nonce string) error {
	i.waitOutFloor(start)

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")

	return c.JSON(http.StatusAccepted, apiauthmodels.RequestOTPResponse{SessionNonce: nonce})
}

// rejected answers a request that issued no code, which covers every failure
// this endpoint can have.
//
// It answers exactly as a successful issue does, nonce included: a caller must
// not be able to tell the two apart, and an absent or empty field would hand
// back in one field the distinction the uniform status exists to remove.
//
// The cause is logged rather than returned, once, here. WARN, not ERROR: a bad
// request to a permanently-live public endpoint is expected traffic, and
// logging it at ERROR would bury the failures that are genuinely the service's
// fault. The response says nothing, so this line is all an operator gets.
func (i *Implementation) rejected(
	ctx context.Context,
	c *echo.Context,
	start time.Time,
	reason string,
	err error,
) error {
	xlog.Warn(ctx, "otp code request rejected",
		xfield.String("reason", reason),
		xfield.Error(err),
	)

	return i.accepted(c, start, placeholderNonce(ctx))
}

// placeholderNonce mints a nonce for a request that issued no code, shaped
// exactly like a real one so the response body does not vary between branches.
func placeholderNonce(ctx context.Context) string {
	nonce, _, err := xcripto.GenerateToken(otpNonceBytes)
	if err == nil {
		return nonce
	}

	// crypto/rand failing is close to impossible, but the fallback still has to
	// hold the shape -- an empty session_nonce would change the body and give
	// the branch away.
	xlog.Error(ctx, "otp nonce generation failed, answering with a placeholder",
		xfield.Error(err),
	)

	// 43 characters plus the "=" that base64.URLEncoding always leaves on 32
	// bytes. A bare run of 44 "A"s would be the one response shape no real nonce
	// can have, which turns a broken RNG into a signal.
	return strings.Repeat("A", otpNonceEncodedLen-1) + "="
}

// waitOutFloor sleeps the remainder of the response floor, if any.
//
// The wait is unconditional -- deliberately not select-ing on ctx.Done(). Honoring
// cancellation here would hand the oracle straight back: a caller that hangs up
// on a timer measures how long the server actually took before the abort, and
// never needs to read a response at all. Since the transaction has already
// committed by this point, what is held is a goroutine, not a database
// connection, and the rate limiter bounds how many of those a caller can stack
// up.
func (i *Implementation) waitOutFloor(start time.Time) {
	remaining := i.otpResponseFloor - time.Since(start)
	if remaining <= 0 {
		return
	}

	time.Sleep(remaining)
}

// validateRequestOTP bounds the address before it reaches the service.
//
// The length cap is not cosmetic: this endpoint is unauthenticated, and an
// unbounded attacker-chosen string would otherwise flow into log lines on every
// rejected request.
func validateRequestOTP(ctx context.Context, body *apiauthmodels.RequestOTPRequest) error {
	return validation.ValidateStructWithContext(ctx, body,
		validation.Field(&body.Email, validation.Required, validation.Length(0, maxEmailLen), is.EmailFormat,
			validation.By(canonicalEmail)),
	)
}
