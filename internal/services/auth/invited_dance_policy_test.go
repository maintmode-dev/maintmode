package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/oauthdance"
)

// stubClaimer stands in for the invitation service, so this test can drive the
// dance's policy decision without the invitation package (which imports this
// one -- the cycle InvitationClaimer exists to break).
type stubClaimer struct {
	resolved   *entity.ResolvedInvitation
	resolveErr error
	claimed    bool
}

func (s *stubClaimer) PrepareHandle(context.Context, string, string) error { return nil }

func (s *stubClaimer) ResolveForIdentity(
	context.Context, string, *entity.OAuthIDTokenClaims,
) (*entity.ResolvedInvitation, error) {
	return s.resolved, s.resolveErr
}

func (s *stubClaimer) ClaimForUser(context.Context, *entity.ResolvedInvitation, uuid.UUID) error {
	s.claimed = true

	return nil
}

// seedAdmin creates an admin so the zero-admin bootstrap branch does not answer
// first. Without it GetOrCreateByAuthInfo grants admin unconditionally and both
// policies would create a user, hiding the gate this test exists to pin.
func seedAdmin(ctx context.Context, t *testing.T, svc *Service) {
	t.Helper()

	_, err := svc.usersSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle,
		&entity.OAuthProviderUserInfo{
			ID:    uuid.NewString(),
			Email: uuid.NewString() + "@admin-gate.test",
			Name:  "Admin",
		}, entity.UserCreationPolicy{AllowCreate: true, GrantRoles: []entity.Role{entity.RoleAdmin}})
	require.NoError(t, err)
}

// TestInvitedDanceAllowCreateGate pins the single most security-relevant line
// in the invited dance: an account is created ONLY when an invitation actually
// resolved.
//
// It builds an INVITE-ONLY service on purpose. The shared local config runs
// auth.allow_open_signup: true, so every handler-level test creates users
// either way and a gate hard-coded to true is invisible there. This is the only
// shape where the gate decides anything.
//
// The stakes are higher than "open signup": GetOrCreateByAuthInfo grants
// RoleAdmin when admins == 0 and checks that BEFORE consulting AllowCreate, so
// on a fresh instance an always-true gate is an open ADMIN signup hole.
func TestInvitedDanceAllowCreateGate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	meta := &entity.AuditMetadata{IP: "127.0.0.1"}

	t.Run("no invitation: the dance refuses to create an account", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initInviteOnlyService(t)
		seedAdmin(ctx, t, svc)

		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), gomock.Any()).
			Return(&entity.OAuthIDTokenClaims{
				Subject: uuid.NewString(),
				Email:   uuid.NewString() + "@uninvited.test",
				Name:    "Uninvited",
			}, nil).
			AnyTimes()

		// No handle, so resolveInvitation returns nil and the policy stays empty.
		_, err := svc.issueDanceCode(ctx, entity.AuthMethodGoogle, "id-token", "", meta)

		require.ErrorIs(t, err, apperr.ErrSignupDisabled,
			"an uninvited dance must not create an account on an invite-only instance")
	})

	t.Run("resolved invitation: the dance creates the account and claims it", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initInviteOnlyService(t)
		seedAdmin(ctx, t, svc)

		claimer := &stubClaimer{resolved: &entity.ResolvedInvitation{
			ID:    uuid.New(),
			Roles: []entity.Role{entity.RoleEditor},
		}}
		svc.WithInvitations(claimer)

		// The success path parks a one-time code, so the store must be armed.
		svc.danceCodes = oauthdance.NewStore(valkey, cfg.Auth.DanceStateTTL())

		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), gomock.Any()).
			Return(&entity.OAuthIDTokenClaims{
				Subject: uuid.NewString(),
				Email:   uuid.NewString() + "@invited.test",
				Name:    "Invited",
			}, nil).
			AnyTimes()

		code, err := svc.issueDanceCode(ctx, entity.AuthMethodGoogle, "id-token", "a-handle", meta)

		require.NoError(t, err, "a resolved invitation must authorize creation")
		assert.NotEmpty(t, code)
		assert.True(t, claimer.claimed, "phase 2 must spend the invitation")
	})
}
