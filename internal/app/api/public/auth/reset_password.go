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

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xemail"
)

// RequestPasswordReset godoc
// @Summary Request a password reset code
// @Description Emails a one-time code, reusing the sign-in code mechanism. Always answers 202 with a session nonce -- for a known address, an unknown one, or a blocked user alike -- so the response cannot be used to discover which accounts exist.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.RequestOTPRequest true "Email"
// @Success 202 {object} apiauthmodels.RequestOTPResponse
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Router /api/v1/password/reset/request [post]
func (i *Implementation) RequestPasswordReset(c *echo.Context) error {
	start := time.Now()

	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.RequestPasswordReset")
	defer span.End()

	body := new(apiauthmodels.RequestOTPRequest)
	if err := c.Bind(body); err != nil {
		return i.rejected(ctx, c, start, "malformed request body", err)
	}

	// Normalized before validation and before the limiter key is derived, so
	// the address the limiter buckets is the address the lookup resolves.
	body.Email = xemail.Normalize(body.Email)

	if err := validateRequestOTP(ctx, body); err != nil {
		return i.rejected(ctx, c, start, "invalid request", err)
	}

	nonce, err := i.authSrv.RequestPasswordReset(ctx, body.Email)
	if err != nil {
		return i.rejected(ctx, c, start, "failed to issue reset code", err)
	}

	return i.accepted(c, start, nonce)
}

// ResetPassword godoc
// @Summary Set a new password with a one-time code
// @Description Redeems a code emailed by /password/reset/request and installs a new password. Revokes EVERY session, including any the caller holds. Answers 204 with no token pair: the caller signs in again with the new password.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.ResetPasswordRequest true "Code, session nonce and new password"
// @Success 204 "Password reset"
// @Failure 401 {object} httperrors.ErrorResponse "Authentication failed, or otp_session_mismatch"
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Router /api/v1/password/reset/confirm [post]
//
// Every failure answers alike -- a wrong code, an expired one, an unknown
// address, and a password that breaks the length policy. A policy failure
// answering differently would tell a caller holding a guessed code that the
// code was right, which is the more valuable secret of the two.
func (i *Implementation) ResetPassword(c *echo.Context) error {
	start := time.Now()

	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.ResetPassword")
	defer span.End()

	body := new(apiauthmodels.ResetPasswordRequest)
	if err := c.Bind(body); err != nil {
		return i.otpRejected(ctx, c, start, "malformed request body", err)
	}

	cmd := &entity.ResetPasswordCmd{
		Email:        xemail.Normalize(body.Email),
		Code:         body.Code,
		SessionNonce: body.SessionNonce,
		NewPassword:  body.NewPassword,
		ClientIP:     c.RealIP(),
		UserAgent:    c.Request().UserAgent(),
	}

	if err := validateResetPasswordCmd(ctx, cmd); err != nil {
		return i.otpRejected(ctx, c, start, "invalid request", err)
	}

	if err := i.authSrv.ResetPassword(ctx, cmd); err != nil {
		if errors.Is(err, apperr.ErrOTPSessionMismatch) {
			return i.otpSessionMismatch(ctx, c, start, err)
		}

		return i.otpRejected(ctx, c, start, "reset rejected", err)
	}

	i.waitOutFloor(start)
	c.Response().Header().Set("Cache-Control", "no-store")

	return c.NoContent(http.StatusNoContent)
}

// validateResetPasswordCmd checks the shape of the request. The new password's
// LENGTH is not checked here: the service applies the policy so that a policy
// failure travels the same path as a bad code and answers identically.
func validateResetPasswordCmd(ctx context.Context, cmd *entity.ResetPasswordCmd) error {
	return validation.ValidateStructWithContext(ctx, cmd,
		validation.Field(&cmd.Email, validation.Required, validation.Length(0, maxEmailLen), is.EmailFormat),
		validation.Field(&cmd.Code, validation.Required, validation.Length(otpCodeLen, otpCodeLen), is.Digit),
		validation.Field(&cmd.SessionNonce, validation.Required, validation.Length(0, maxSessionNonceLen)),
		validation.Field(&cmd.NewPassword, validation.Required),
		validation.Field(&cmd.ClientIP, validation.Required),
	)
}
