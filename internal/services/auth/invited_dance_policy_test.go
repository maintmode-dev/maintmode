package auth

import (
	"context"
	"slices"
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

func (s *stubClaimer) ResolveByToken(context.Context, string) (*entity.Invitation, error) {
	return nil, apperr.ErrInvalidInvitation
}

// ClaimForUser returns the user holding the invitation's roles on top of the
// defaults, as the real claimer's AssignRoles does.
func (s *stubClaimer) ClaimForUser(
	_ context.Context, inv *entity.ResolvedInvitation, userID uuid.UUID,
) (*entity.User, error) {
	s.claimed = true

	return &entity.User{ID: userID, Roles: append(slices.Clone(entity.DefaultRoles), inv.Roles...)}, nil
}

// TestInvitedDanceAllowCreateGate pins the single most security-relevant line
// in the invited dance: an account is created ONLY when an invitation actually
// resolved.
//
// It builds an INVITE-ONLY service on purpose. The shared local config runs
// auth.allow_open_signup: true, so every handler-level test creates users
// either way and a gate hard-coded to true is invisible there. This is the only
// shape where the gate decides anything.
func TestInvitedDanceAllowCreateGate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	meta := &entity.AuditMetadata{IP: "127.0.0.1"}

	t.Run("no invitation: the dance refuses to create an account", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initInviteOnlyService(t)

		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), gomock.Any()).
			Return(&entity.OAuthIDTokenClaims{
				Subject: uuid.NewString(),
				Email:   uuid.NewString() + "@uninvited.test",
				Name:    "Uninvited",
			}, nil).
			AnyTimes()

		// No handle, so resolveInvitation returns nil and the policy stays empty.
		_, err := svc.issueDanceCode(ctx, entity.AuthMethodGoogle, "id-token", "", "", meta)

		require.ErrorIs(t, err, apperr.ErrSignupDisabled,
			"an uninvited dance must not create an account on an invite-only instance")
	})

	t.Run("resolved invitation: the dance creates the account and claims it", func(t *testing.T) {
		t.Parallel()
		claimer := &stubClaimer{resolved: &entity.ResolvedInvitation{
			ID:    uuid.New(),
			Roles: []entity.Role{entity.RoleEditor},
		}}
		// The success path parks a one-time code, so the store must be armed.
		svc, mocks := initServiceWithDeps(t, entity.AuthMethodGoogle, serviceDeps{
			inviteOnly:  true,
			codes:       oauthdance.NewStore(valkey, cfg.Auth.DanceStateTTL()),
			invitations: claimer,
		})

		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), gomock.Any()).
			Return(&entity.OAuthIDTokenClaims{
				Subject: uuid.NewString(),
				Email:   uuid.NewString() + "@invited.test",
				Name:    "Invited",
			}, nil).
			AnyTimes()

		outcome, err := svc.issueDanceCode(ctx, entity.AuthMethodGoogle, "id-token", "", "a-handle", meta)

		require.NoError(t, err, "a resolved invitation must authorize creation")
		require.NotNil(t, outcome)
		assert.NotEmpty(t, outcome.Code)
		assert.False(t, outcome.Linked, "an invited sign-in is not a link")
		assert.True(t, claimer.claimed, "phase 2 must spend the invitation")
	})
}

// TestInvitedDanceTokenCarriesInvitationRoles pins the order of the invited
// dance: the invitation's roles are granted BEFORE the pair is minted. The
// access token carries the roles the user holds at minting, so a pair issued
// first hands an invited admin a guest token, and every admin endpoint answers
// 403 until the next refresh -- while /me, which reads the database, already
// shows the admin.
func TestInvitedDanceTokenCarriesInvitationRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	claimer := &stubClaimer{resolved: &entity.ResolvedInvitation{
		ID:    uuid.New(),
		Roles: []entity.Role{entity.RoleAdmin},
	}}
	codes := oauthdance.NewStore(valkey, cfg.Auth.DanceStateTTL())
	svc, mocks := initServiceWithDeps(t, entity.AuthMethodGoogle, serviceDeps{
		inviteOnly:  true,
		codes:       codes,
		invitations: claimer,
	})

	mocks.authMethod.EXPECT().
		Authenticate(gomock.Any(), gomock.Any()).
		Return(&entity.OAuthIDTokenClaims{
			Subject: uuid.NewString(),
			Email:   uuid.NewString() + "@invited.test",
			Name:    "Invited admin",
		}, nil).
		AnyTimes()

	outcome, err := svc.issueDanceCode(ctx, entity.AuthMethodGoogle, "id-token", "", "a-handle",
		&entity.AuditMetadata{IP: "127.0.0.1"})
	require.NoError(t, err)

	pair, err := codes.ConsumeCode(ctx, outcome.Code)
	require.NoError(t, err)

	access, err := svc.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
	require.NoError(t, err)
	require.Contains(t, access.UserRoles, entity.RoleAdmin,
		"the access token must carry the invitation's roles")
}
