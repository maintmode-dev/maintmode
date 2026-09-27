package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

// hasBuiltinSignIn reports whether the user could still sign in with no
// provider linked at all, which is what decides whether their last provider
// may be disconnected.
//
// Only a method that would actually let them in counts. An emailed code needs
// nothing but the account's address, so email_otp counts whenever it is
// offered. That assumes the instance can send the code: nothing ties the flag
// to a working email integration, so an operator who offers codes without one
// leaves this check trusting a method that cannot deliver. A password counts
// only while email_password is offered AND the user has set one: a password
// the instance no longer accepts is not a way in, and counting it would let
// someone disconnect their last provider and lock themselves out.
//
// Break-glass never counts: it belongs to the deployment, not to the account.
//
// A flag or credential that cannot be read fails the disconnect rather than
// guessing -- the answer decides whether an account can be left without a way
// in.
func (s *Service) hasBuiltinSignIn(ctx context.Context, userID uuid.UUID) (bool, error) {
	otpOffered, err := s.methodFlags.Enabled(ctx, entity.AuthMethodNameEmailOTP)
	if err != nil {
		return false, fmt.Errorf("read email_otp flag: %w", err)
	}
	if otpOffered {
		return true, nil
	}

	passwordOffered, err := s.methodFlags.Enabled(ctx, entity.AuthMethodNameEmailPassword)
	if err != nil {
		return false, fmt.Errorf("read email_password flag: %w", err)
	}
	if !passwordOffered {
		return false, nil
	}

	return s.HasPassword(ctx, userID)
}
