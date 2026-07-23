package invitation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestCreate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initService(t)
		emailAddr := uniqueEmail(t)

		inv, err := svc.Create(ctx, &entity.CreateInvitationCmd{
			Actor: makeAdmin(ctx, t, svc),
			Email: emailAddr,
			Roles: []entity.Role{entity.RoleEditor},
		})
		require.NoError(t, err)
		require.Equal(t, emailAddr, inv.Email)
		require.Equal(t, entity.InvitationStatusPending, inv.Status)
		require.Equal(t, []entity.Role{entity.RoleEditor}, inv.Roles)
		require.WithinDuration(t, time.Now().UTC().Add(testInvitationTTL), inv.ExpiresAt, time.Minute)
		// Email "sent" to the invitee with a link carrying the raw token.
		require.Equal(t, emailAddr, mocks.sentEmail.target)
		require.Contains(t, mocks.sentEmail.body, "/accept-invite?token=")
	})

	t.Run("empty roles rejected", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)

		_, err := svc.Create(ctx, &entity.CreateInvitationCmd{
			Actor: makeAdmin(ctx, t, svc),
			Email: uniqueEmail(t),
			Roles: nil,
		})
		require.ErrorIs(t, err, apperr.ErrValidation)
	})

	t.Run("invalid role rejected", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)

		_, err := svc.Create(ctx, &entity.CreateInvitationCmd{
			Actor: makeAdmin(ctx, t, svc),
			Email: uniqueEmail(t),
			Roles: []entity.Role{"superuser"},
		})
		require.ErrorIs(t, err, apperr.ErrInvalidRole)
	})

	t.Run("existing user rejected with conflict", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)

		existing := makeAdmin(ctx, t, svc) // a real user

		_, err := svc.Create(ctx, &entity.CreateInvitationCmd{
			Actor: makeAdmin(ctx, t, svc),
			Email: existing.Email,
			Roles: []entity.Role{entity.RoleEditor},
		})
		require.ErrorIs(t, err, apperr.ErrUserAlreadyExists)
	})

	t.Run("re-invite revokes the prior pending and creates a new one", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		emailAddr := uniqueEmail(t)

		first := mustCreate(ctx, t, svc, emailAddr)
		second := mustCreate(ctx, t, svc, emailAddr)

		require.NotEqual(t, first.ID, second.ID)
		require.NotEqual(t, first.TokenHash, second.TokenHash)

		// The first invitation is now revoked; the second is the live one.
		got, err := svc.store.GetByID(ctx, first.ID)
		require.NoError(t, err)
		require.Equal(t, entity.InvitationStatusRevoked, got.Status)
	})

	t.Run("seat-role invite at full cap is rejected", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initService(t)
		mocks.seatGuard.err = apperr.ErrSeatsLimitExceeded // license reports the cap is full

		_, err := svc.Create(ctx, &entity.CreateInvitationCmd{
			Actor: makeAdmin(ctx, t, svc),
			Email: uniqueEmail(t),
			Roles: []entity.Role{entity.RoleEditor},
		})
		require.ErrorIs(t, err, apperr.ErrSeatsLimitExceeded)
		require.Equal(t, 1, mocks.seatGuard.called, "a seat-role invite must consult the guard")
	})

	t.Run("guest invite passes even at full cap", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initService(t)
		mocks.seatGuard.err = apperr.ErrSeatsLimitExceeded // cap full, but a guest takes no seat

		inv, err := svc.Create(ctx, &entity.CreateInvitationCmd{
			Actor: makeAdmin(ctx, t, svc),
			Email: uniqueEmail(t),
			Roles: []entity.Role{entity.RoleGuest},
		})
		require.NoError(t, err)
		require.Equal(t, []entity.Role{entity.RoleGuest}, inv.Roles)
		require.Zero(t, mocks.seatGuard.called, "a guest invite must not consult the seat guard")
	})

	t.Run("seat-role invite passes when the guard reports room", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initService(t)
		// seatGuard.err stays nil: the last seat is grantable.

		_, err := svc.Create(ctx, &entity.CreateInvitationCmd{
			Actor: makeAdmin(ctx, t, svc),
			Email: uniqueEmail(t),
			Roles: []entity.Role{entity.RoleReviewer},
		})
		require.NoError(t, err)
		require.Equal(t, 1, mocks.seatGuard.called)
	})
}
