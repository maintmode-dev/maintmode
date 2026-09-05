package otp

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
	"github.com/ruko1202/maintmode/internal/utils/xcripto"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// sessionNonceBytes is the entropy of the session nonce: 256 bits, matching the
// invitation token generator.
const sessionNonceBytes = 32

// Request issues a one-time code for an email address and queues its delivery.
//
// It always returns a nonce and never reports whether the address belongs to
// anyone. That is the point: this endpoint is unauthenticated, the instance is
// invite-only, and "does this account exist" is worth extracting on its own. A
// caller cannot tell an issued code from a silent no-op, and the nonce is
// returned on both paths so its presence is not the tell.
//
// The nonce binds the attempt to whoever started it. It is minted here and
// stored beside the code's hash, and Verify compares it. It is never emailed --
// that separation is the control: someone who talks a victim into reading out
// the code still does not have it.
func (s *Service) Request(ctx context.Context, email string) (nonce string, err error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.OTP.Request")
	defer span.End()

	// GenerateToken's second return is its sha256; the nonce is stored raw and
	// the digest is not used. Raw because the nonce is not a bearer credential:
	// it proves nothing on its own, it is never emailed, and someone holding it
	// still needs the code. Hashing it would protect nothing the code's own hash
	// and its short lifetime do not already cover.
	nonce, _, err = xcripto.GenerateToken(sessionNonceBytes)
	if err != nil {
		return "", fmt.Errorf("generate session nonce: %w", err)
	}

	user, err := s.userSrv.GetByEmail(ctx, email)
	switch {
	case errors.Is(err, apperr.ErrUserNotFound):
		// No account. No row, no email, same nonce, same answer.
		xlog.Info(ctx, "otp code not issued: no such user")
		return nonce, nil
	case err != nil:
		return "", fmt.Errorf("resolve user by email: %w", err)
	}

	if user.IsBlocked() {
		// The RESPONSE is indistinguishable from the branch above by design:
		// telling the two apart would leak both that the account exists and what
		// state it is in. The logs do differ, deliberately -- an operator holding
		// them has already earned the distinction, and a blocked account trying
		// to sign in is worth a warning.
		xlog.Warn(ctx, "otp code not issued: user is blocked",
			xfield.String("user_id", user.ID.String()),
		)
		return nonce, nil
	}

	issued, err := s.issue(ctx, user, nonce)
	if err != nil {
		return "", err
	}

	if !issued {
		// The user's live code has spent its guess ceiling and has not yet
		// expired. Issuing now would hand back a fresh code with a fresh
		// counter, which turns "five attempts" into "five attempts per code,
		// unlimited codes" -- the ceiling would buy nothing.
		//
		// Answered exactly like the two branches above: same nonce shape, no
		// error, so the handler's ordinary 202 path runs. A caller must not be
		// able to tell "barred right now" from "unknown address" or "a code is
		// on its way" -- the recovery is the same in all three, and the
		// difference is worth something only to someone probing addresses.
		//
		// INFO, not WARN, and no audit row: no secret was presented, so this is
		// not a sign-in attempt. The five failures that produced the bar are
		// already on record, and auditing the refusal would let anyone write
		// unbounded rows by replaying this endpoint against a known address.
		xlog.Info(ctx, "otp code not issued: live code has spent its attempts",
			xfield.String("user_id", user.ID.String()),
		)
	}

	return nonce, nil
}

// issue writes the code and queues its delivery in one transaction, so a
// credential never outlives the task meant to deliver it and vice versa.
//
// It reports whether a code was actually issued. False is not a failure: it
// means the slot is held by a live code that has spent its ceiling, and the
// caller answers exactly as it does for a successful issue.
//
// The flag is carried out through a captured variable rather than a return from
// the closure, because WithinTx takes func(ctx) error and commits on nil -- so
// there is nowhere in the closure's signature to put it. Signaling with a
// sentinel error instead would reach the handler's rejected() path, which logs
// at WARN on every call; this branch is replayable by anyone who knows an
// address, so that would bury real failures under routine traffic.
func (s *Service) issue(ctx context.Context, user *entity.User, nonce string) (bool, error) {
	code, codeHash, err := xcripto.GenerateOTPCode()
	if err != nil {
		return false, fmt.Errorf("generate otp code: %w", err)
	}

	issued := false

	err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		free, err := s.claimSlot(ctx, user.ID)
		if err != nil {
			return err
		}

		if !free {
			// Nothing to roll back: the read took a lock and wrote nothing, so
			// the transaction commits empty and `issued` stays false.
			return nil
		}

		expiresAt := xtime.UTCNow().Add(s.ttl)

		cred, err := s.store.Create(ctx, &entity.AuthCredential{
			UserID:       user.ID,
			Kind:         entity.AuthCredentialKindOTP,
			SecretHash:   codeHash,
			ExpiresAt:    &expiresAt,
			SessionNonce: &nonce,
		})
		if err != nil {
			// A conflict here means a concurrent request won the race after this
			// one freed the slot. Not retried: that request has already sent this
			// user a code, and a second one is worse than none. The rollback also
			// undoes this transaction's consume, so the winner's row stands.
			return fmt.Errorf("create otp credential: %w", err)
		}

		// Sealed only now: the AAD binds the envelope to the credential id, and
		// the id is generated by the database and known only from the insert.
		// Minting one in Go instead would fail silently -- the column is not
		// insertable, so the row would carry a different id than the AAD claims,
		// and every delivery would fail its tag check.
		task, err := s.sealForDelivery(cred, user.Email, code)
		if err != nil {
			return err
		}

		// Joins this transaction, so the code row and its delivery task commit
		// together. A failed enqueue -- or a failed wrap above, which calls the
		// KMS -- rolls the credential back rather than persisting a code nobody
		// will ever be sent.
		if _, err := s.scheduler.Schedule(
			ctx,
			entity.ProcessorTaskOTPEmailSend,
			task,
			idempotencyKey(cred.ID.String()),
		); err != nil {
			return fmt.Errorf("enqueue otp email: %w", err)
		}

		xlog.Info(ctx, "otp code issued",
			xfield.String("user_id", user.ID.String()),
			xfield.String("credential_id", cred.ID.String()),
		)

		issued = true

		return nil
	})

	return issued, err
}

// claimSlot frees the single active-OTP slot for a new code, and reports whether
// the caller may use it.
//
// It refuses exactly one case: a live code that is unexpired and has spent its
// guess ceiling. That code keeps the slot until it dies of its own accord, so
// burning the attempts on a code cannot be undone by simply asking for another.
// The bar therefore lasts at most the code's remaining lifetime -- deliberately
// not longer, since a durable lock on an endpoint anyone can call against any
// address would be a denial-of-service primitive rather than a control.
//
// An expired burnt code is retired like any other: the ceiling protects a code
// that can still be guessed, and an expired one cannot.
//
// The read takes a row lock. Without it a verify claiming the final attempt
// concurrently would let this read see a count one short of the ceiling, judge
// the code still usable, and free the slot -- handing back the fresh code with a
// fresh counter that the ceiling exists to deny.
//
// Consuming rather than deleting mirrors how invitation reissue revokes the
// previous row: the superseded attempt stays visible in the table.
func (s *Service) claimSlot(ctx context.Context, userID uuid.UUID) (bool, error) {
	live, err := s.store.GetUnconsumedOTPByUserIDForUpdate(ctx, userID)
	switch {
	case errors.Is(err, apperr.ErrAuthCredentialNotFound):
		// The ordinary first-request case, not a failure.
		return true, nil
	case err != nil:
		return false, fmt.Errorf("look up live otp: %w", err)
	}

	if live.Attempts >= s.maxAttempts && live.ExpiresAt != nil && live.ExpiresAt.After(xtime.UTCNow()) {
		return false, nil
	}

	// A false here means a concurrent request consumed the same row first. Either
	// way the slot is free, which is all this step needs.
	if _, err := s.store.ConsumeOTP(ctx, live.ID); err != nil {
		return false, fmt.Errorf("consume live otp: %w", err)
	}

	return true, nil
}

// sealForDelivery builds the delivery task: the code under a fresh data key,
// that key under the active KEK.
//
// The data key is per-task and never persisted, so it lives exactly as long as
// the task does. It buys protection against a dump, a replica or a backup --
// goque_task is never pruned, so the row outlives the code indefinitely -- and
// not against anyone holding the KEK, which opens the wrapped key traveling
// beside it. That is the same exposure a long-lived named key would carry, which
// is why the extra machinery was not worth buying.
func (s *Service) sealForDelivery(
	cred *entity.AuthCredential,
	target, code string,
) (entity.ProcessorTaskPayloadOTPEmail, error) {
	dek, err := secrets.GenerateDEK()
	if err != nil {
		return entity.ProcessorTaskPayloadOTPEmail{}, fmt.Errorf("generate otp data key: %w", err)
	}

	envelope, err := s.cipher.Encrypt(dek, []byte(code), secrets.OTPCodeAAD(cred.ID.String()))
	if err != nil {
		return entity.ProcessorTaskPayloadOTPEmail{}, fmt.Errorf("seal otp code: %w", err)
	}

	wrapped, kekURI, err := s.keyring.WrapDEK(dek)
	if err != nil {
		return entity.ProcessorTaskPayloadOTPEmail{}, fmt.Errorf("wrap otp data key: %w", err)
	}

	return entity.ProcessorTaskPayloadOTPEmail{
		CredentialID: cred.ID,
		Target:       target,
		Code:         envelope,
		DEK:          wrapped,
		// Carried because UnwrapDEK addresses keys by URI: without it a rotation
		// between enqueue and drain strands every task in flight.
		KEKURI:    kekURI,
		ExpiresAt: *cred.ExpiresAt,
	}, nil
}

// idempotencyKey keys the delivery task on the credential id.
//
// The id is uuidv7-generated by the database, so this key can never collide --
// which is the property being bought. goque's unique (type, external_id) index
// raises inside the transaction, so a key that could collide would roll back the
// credential itself; that is the trap the invitation path documents when it
// explains why it keys on a token hash rather than a timestamp.
//
// It follows that a retried request is not collapsed: it mints a new credential,
// a new key and a second email. Only the newest code works, which is why the
// email copy points at the newest one.
func idempotencyKey(credentialID string) string {
	return xhash.HashSha256(fmt.Appendf(nil, "otp-email:%s", credentialID))
}
