package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_user "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/user"
	"github.com/ruko1202/maintmode/internal/services/invitation/claimer"
	"github.com/ruko1202/maintmode/internal/services/license"
	"github.com/ruko1202/maintmode/internal/services/user"
	"github.com/ruko1202/maintmode/internal/storages/authcredentials"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/userinvitations"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

const invitePassword = "a-long-enough-password"

type invitePasswordHarness struct {
	svc         *Service
	invitations *userinvitations.Store
	audit       *recordingAuditPublisher
}

// initInvitePasswordService builds an invite-only service over the REAL
// claimer, so claiming writes real rows inside the acceptance transaction.
// wrap, when set, decorates the claimer.
func initInvitePasswordService(
	t *testing.T, flags AuthMethodFlags, wrap func(InvitationClaimer) InvitationClaimer,
) *invitePasswordHarness {
	t.Helper()

	txManager := dbtx.NewTxManager(db)
	store := userinvitations.NewStore(db)
	claimerUsers := user.NewService(
		txManager,
		users.NewStore(db),
		useridentities.NewStore(db),
		newTestAuditPublisher(t),
		mock_user.NewMockTokenRevoker(gomock.NewController(t)),
		license.NewNoop(),
		false,
		loginProviders,
	)

	var invitations InvitationClaimer = claimer.New(txManager, store, claimerUsers, nil)
	if wrap != nil {
		invitations = wrap(invitations)
	}

	svc, _ := initServiceWithDeps(t, entity.AuthMethodGoogle, serviceDeps{
		inviteOnly:  true,
		invitations: invitations,
		flags:       flags,
	})
	rec := newRecordingAuditPublisher()
	svc.auditPublisher = rec

	return &invitePasswordHarness{svc: svc, invitations: store, audit: rec}
}

// seedInvitation stores a live invitation for a fresh address and returns it
// with its raw link token.
func (h *invitePasswordHarness) seedInvitation(
	ctx context.Context, t *testing.T, roles ...entity.Role,
) (inv *entity.Invitation, rawToken string) {
	t.Helper()

	inviter, err := h.svc.usersSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "@inviter.test",
		Name:  "Inviter",
	}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	rawToken = xuuid.NewString()
	inv, err = h.invitations.Create(ctx, &entity.Invitation{
		Email:       xuuid.NewString() + "@invitee.test",
		Roles:       roles,
		TokenHash:   xhash.HashSha256([]byte(rawToken)),
		Status:      entity.InvitationStatusPending,
		ExpiresAt:   xtime.UTCNow().Add(time.Hour),
		SentAt:      xtime.UTCNow().Add(-time.Minute),
		InvitedByID: inviter.ID,
	})
	require.NoError(t, err)

	return inv, rawToken
}

func (h *invitePasswordHarness) accept(
	ctx context.Context, rawToken, password string,
) (*entity.TokenPair, error) {
	return h.svc.AcceptInvitationWithPassword(ctx, &entity.AcceptInvitationWithPasswordCmd{
		Token:     rawToken,
		Password:  password,
		ClientIP:  "10.0.0.1",
		UserAgent: "test-agent",
	})
}

func (h *invitePasswordHarness) requirePending(ctx context.Context, t *testing.T, id uuid.UUID) {
	t.Helper()

	inv, err := h.invitations.GetByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, entity.InvitationStatusPending, inv.Status, "a refused acceptance must not spend the invitation")
}

func requireNoUser(ctx context.Context, t *testing.T, email string) {
	t.Helper()

	_, err := users.NewStore(db).GetByEmail(ctx, email)
	require.ErrorIs(t, err, apperr.ErrUserNotFound, "a refused acceptance must leave no account behind")
}

func TestAcceptInvitationWithPassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("creates the invited account with a working password and the invitation's roles", func(t *testing.T) {
		t.Parallel()
		h := initInvitePasswordService(t, nil, nil)
		inv, rawToken := h.seedInvitation(ctx, t, entity.RoleEditor)

		pair, err := h.accept(ctx, rawToken, invitePassword)
		require.NoError(t, err)

		access, err := h.svc.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.Contains(t, access.UserRoles, entity.RoleEditor, "the token must carry the invitation's roles")

		created, err := users.NewStore(db).GetByEmail(ctx, inv.Email)
		require.NoError(t, err)
		require.Equal(t, inv.Email, created.Email)

		accepted, err := h.invitations.GetByID(ctx, inv.ID)
		require.NoError(t, err)
		require.Equal(t, entity.InvitationStatusAccepted, accepted.Status)

		// The password is real: it signs the new account in.
		_, err = h.svc.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: inv.Email, Password: invitePassword, ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		var success *audit.LoginSuccess
		for _, action := range h.audit.actions() {
			if s, ok := action.(audit.LoginSuccess); ok && s.User.ID == created.ID {
				success = &s
				break
			}
		}
		require.NotNil(t, success, "the acceptance is a sign-in and must be audited as one")
		require.Equal(t, entity.AuditLoginMethodPassword, success.Meta.LoginMethod)
		require.Equal(t, pair.SessionID.String(), success.Meta.SessionID)
	})

	t.Run("a second acceptance with the same link is refused", func(t *testing.T) {
		t.Parallel()
		h := initInvitePasswordService(t, nil, nil)
		_, rawToken := h.seedInvitation(ctx, t, entity.RoleEditor)

		_, err := h.accept(ctx, rawToken, invitePassword)
		require.NoError(t, err)

		_, err = h.accept(ctx, rawToken, invitePassword)
		require.ErrorIs(t, err, apperr.ErrInvalidInvitation)
	})

	t.Run("an unknown token is refused as invalid", func(t *testing.T) {
		t.Parallel()
		h := initInvitePasswordService(t, nil, nil)

		_, err := h.accept(ctx, xuuid.NewString(), invitePassword)
		require.ErrorIs(t, err, apperr.ErrInvalidInvitation)
	})

	t.Run("a password outside the policy is a validation error and changes nothing", func(t *testing.T) {
		t.Parallel()
		h := initInvitePasswordService(t, nil, nil)
		inv, rawToken := h.seedInvitation(ctx, t, entity.RoleEditor)

		_, err := h.accept(ctx, rawToken, "too-short")
		require.ErrorIs(t, err, apperr.ErrValidation)
		require.NotErrorIs(t, err, apperr.ErrInvalidInvitation)

		h.requirePending(ctx, t, inv.ID)
		requireNoUser(ctx, t, inv.Email)
	})

	t.Run("refused while email_password is disabled", func(t *testing.T) {
		t.Parallel()
		h := initInvitePasswordService(t, flagsWith(map[entity.AuthMethodName]bool{
			entity.AuthMethodNameEmailPassword: false,
		}), nil)
		inv, rawToken := h.seedInvitation(ctx, t, entity.RoleEditor)

		_, err := h.accept(ctx, rawToken, invitePassword)
		require.ErrorIs(t, err, apperr.ErrSignInMethodDisabled)

		h.requirePending(ctx, t, inv.ID)
		requireNoUser(ctx, t, inv.Email)
	})

	t.Run("never attaches a password to an account that already exists", func(t *testing.T) {
		t.Parallel()
		h := initInvitePasswordService(t, nil, nil)
		inv, rawToken := h.seedInvitation(ctx, t, entity.RoleEditor)

		existing, err := h.svc.usersSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
			ID: xuuid.NewString(), Email: inv.Email, Name: "Already here",
		}, entity.UserCreationPolicy{AllowCreate: true})
		require.NoError(t, err)

		_, err = h.accept(ctx, rawToken, invitePassword)
		require.ErrorIs(t, err, apperr.ErrUserAlreadyExists)

		_, err = authcredentials.NewStore(db).GetPasswordByUserID(ctx, existing.ID)
		require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound, "the existing account must not gain a password")
		h.requirePending(ctx, t, inv.ID)
	})

	t.Run("losing the claim to a concurrent acceptance rolls the new account back", func(t *testing.T) {
		t.Parallel()
		var store *userinvitations.Store
		h := initInvitePasswordService(t, nil, func(inner InvitationClaimer) InvitationClaimer {
			return &raceLosingClaimer{InvitationClaimer: inner, store: func() *userinvitations.Store { return store }}
		})
		store = h.invitations
		inv, rawToken := h.seedInvitation(ctx, t, entity.RoleEditor)

		_, err := h.accept(ctx, rawToken, invitePassword)
		require.ErrorIs(t, err, apperr.ErrInvalidInvitation)

		requireNoUser(ctx, t, inv.Email)
	})
}

// raceLosingClaimer reproduces a concurrent acceptance deterministically: just
// before this acceptance claims, another one wins -- the invitation is marked
// accepted on a separate connection, committed, outside this acceptance's
// transaction. The real claim then finds nothing pending.
type raceLosingClaimer struct {
	InvitationClaimer
	store func() *userinvitations.Store
}

func (c *raceLosingClaimer) ClaimForUser(
	ctx context.Context, inv *entity.ResolvedInvitation, userID uuid.UUID,
) (*entity.User, error) {
	won, err := c.store().MarkAccepted(context.Background(), inv.ID)
	if err != nil {
		return nil, err
	}
	if !won {
		return nil, apperr.ErrInvalidInvitation
	}

	return c.InvitationClaimer.ClaimForUser(ctx, inv, userID)
}
