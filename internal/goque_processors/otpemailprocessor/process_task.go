package otpemailprocessor

import (
	"context"
	"fmt"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
	"github.com/ruko1202/maintmode/internal/services/otp"
)

// ProcessTask opens the sealed code and emails it.
//
// Three failures are possible and each is logged distinctly, because from
// outside they are indistinguishable: the endpoint answered 202 in every case,
// so these log lines are the only signal that codes are not arriving. Nothing
// alerts on them yet.
func (p *queueProcessor) ProcessTask(
	ctx context.Context,
	task *goque.TypedTask[entity.ProcessorTaskPayloadOTPEmail],
) error {
	payload := task.Payload

	ctx, span := xlog.WithOperationSpan(ctx, "processor.OTPEmail.ProcessTask",
		xfield.String("credential_id", payload.CredentialID.String()),
	)
	defer span.End()

	// Stop before sending a code that is already dead. Nothing overrides goque's
	// retry period, which is longer than the code's lifetime, so any retry at all
	// lands here -- in practice a failed send gets one attempt, not max_attempts.
	// Delivering an expired code reads to the recipient as a broken product.
	//
	// Cancel rather than return nil: a nil would record an undelivered
	// authentication code as successfully processed, erasing the one trace an
	// operator has. The task keeps status=canceled; goque drops the error text
	// unless it also wraps a decode failure, so the reason lives in this log line.
	if !p.now().Before(payload.ExpiresAt) {
		xlog.Warn(ctx, "otp email dropped: code expired before delivery",
			xfield.Time("expires_at", payload.ExpiresAt),
		)
		return fmt.Errorf("%w: otp code expired before delivery", goque.ErrTaskCancel)
	}

	dek, err := p.keyring.UnwrapDEK(payload.DEK, payload.KEKURI)
	if err != nil {
		// Typically a KEK that the keyring no longer serves: a rotation flipped
		// the active URI and this task predates it. Retrying cannot help, and the
		// task will expire out above.
		xlog.Error(ctx, "otp email failed: cannot unwrap the task data key",
			xfield.String("kek_uri", payload.KEKURI),
			xfield.Error(err),
		)
		return fmt.Errorf("unwrap otp dek: %w", err)
	}

	code, err := p.cipher.Decrypt(dek, payload.Code, secrets.OTPCodeAAD(payload.CredentialID.String()))
	if err != nil {
		xlog.Error(ctx, "otp email failed: cannot decrypt the code", xfield.Error(err))
		return fmt.Errorf("decrypt otp code: %w", err)
	}

	body, err := otp.RenderOTPEmail(string(code), p.ttl)
	if err != nil {
		xlog.Error(ctx, "otp email failed: cannot render the body", xfield.Error(err))
		return fmt.Errorf("render otp email: %w", err)
	}

	// HTML, so the transport applies its branded frame -- it only wraps the HTML
	// path, and a plain-text message would ship unframed.
	_, err = p.sender.Send(ctx,
		entity.NotifyTransportEmail,
		payload.Target,
		entity.NotifyMessage{
			Subject:     otp.OTPEmailSubject,
			Body:        body,
			MessageMIME: entity.HTMLMessageMIME,
		},
		// A sign-in code is not a reply to anything.
		nil,
	)
	if err != nil {
		xlog.Error(ctx, "otp email failed: transport rejected the message", xfield.Error(err))
		return fmt.Errorf("send otp email: %w", err)
	}

	return nil
}
