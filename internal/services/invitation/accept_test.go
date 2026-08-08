package invitation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_invitation "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/invitation"
	mock_oauthprovider "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/oauthprovider"
	mock_user "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/user"
	"github.com/ruko1202/maintmode/internal/services/license"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
	"github.com/ruko1202/maintmode/internal/services/user"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/userinvitations"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

func TestAcceptGuards(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("unknown token is invalid", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)

		_, err := svc.Accept(ctx, &entity.AcceptInvitationCmd{
			Token:    "missing",
			Provider: entity.OAuthProviderGoogle,
			IDToken:  "tok",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidInvitation)
	})

	t.Run("revoked token is invalid", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initService(t)
		inv := mustCreate(ctx, t, svc, uniqueEmail(t))
		require.NoError(t, svc.Revoke(ctx, &entity.RevokeInvitationCmd{Actor: makeAdmin(ctx, t, svc), ID: inv.ID}))
		raw := rawTokenFromLink(t, mocks.sentEmail.body)

		_, err := svc.Accept(ctx, &entity.AcceptInvitationCmd{
			Token:    raw,
			Provider: entity.OAuthProviderGoogle,
			IDToken:  "tok",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidInvitation)
	})

	t.Run("email mismatch is rejected without issuing a token", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initService(t)
		mustCreate(ctx, t, svc, uniqueEmail(t))
		raw := rawTokenFromLink(t, mocks.sentEmail.body)

		// OAuth verifies, but resolves to a different email than the invite.
		mocks.oauthProvider.EXPECT().VerifyToken(gomock.Any(), "tok").Return(&entity.OAuthIDTokenClaims{
			Subject: newUUID().String(),
			Email:   "someone-else@evil.com",
			Name:    "Mallory",
		}, nil)
		// IssueTokenPair must NOT be called — no EXPECT() set, so gomock fails the
		// test if the flow reaches token issuance.

		_, err := svc.Accept(ctx, &entity.AcceptInvitationCmd{
			Token:    raw,
			Provider: entity.OAuthProviderGoogle,
			IDToken:  "tok",
		})
		require.ErrorIs(t, err, apperr.ErrEmailMismatch)
	})
}

func TestAcceptSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, mocks := initService(t)
	emailAddr := uniqueEmail(t)
	mustCreate(ctx, t, svc, emailAddr, entity.RoleReviewer)
	raw := rawTokenFromLink(t, mocks.sentEmail.body)

	mocks.oauthProvider.EXPECT().VerifyToken(gomock.Any(), "tok").Return(&entity.OAuthIDTokenClaims{
		Subject: newUUID().String(),
		Email:   emailAddr,
		Name:    "Invited User",
	}, nil)

	// Capture the user handed to IssueTokenPair to assert it carries the
	// invitation's pre-assigned role.
	var issuedFor *entity.User
	mocks.tokenIssuer.EXPECT().
		IssueTokenPair(gomock.Any(), gomock.Any(), "127.0.0.1").
		DoAndReturn(func(_ context.Context, u *entity.User, _ string) (*entity.TokenPair, error) {
			issuedFor = u
			return &entity.TokenPair{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 60}, nil
		})

	pair, err := svc.Accept(ctx, &entity.AcceptInvitationCmd{
		Token:    raw,
		Provider: entity.OAuthProviderGoogle,
		IDToken:  "tok",
		ClientIP: "127.0.0.1",
	})
	require.NoError(t, err)
	require.Equal(t, "access", pair.AccessToken)
	require.NotNil(t, issuedFor)
	require.Contains(t, issuedFor.Roles, entity.RoleReviewer)

	// The invitation is now accepted, so a second accept fails as invalid.
	_, err = svc.Accept(ctx, &entity.AcceptInvitationCmd{
		Token:    raw,
		Provider: entity.OAuthProviderGoogle,
		IDToken:  "tok",
	})
	require.ErrorIs(t, err, apperr.ErrInvalidInvitation)
}

// TestAcceptConcurrentSingleUse asserts that two simultaneous accepts of the
// same valid token result in exactly one success — the status-guarded
// MarkAccepted claim is the authoritative single-use gate.
func TestAcceptConcurrentSingleUse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, mocks := initService(t)
	emailAddr := uniqueEmail(t)
	mustCreate(ctx, t, svc, emailAddr, entity.RoleEditor)
	raw := rawTokenFromLink(t, mocks.sentEmail.body)

	// Both racers verify OAuth to the invited email; the DB claim decides.
	mocks.oauthProvider.EXPECT().VerifyToken(gomock.Any(), "tok").Return(&entity.OAuthIDTokenClaims{
		Subject: newUUID().String(),
		Email:   emailAddr,
		Name:    "Invited User",
	}, nil).AnyTimes()
	// Only the winner reaches token issuance; the loser fails before it.
	mocks.tokenIssuer.EXPECT().
		IssueTokenPair(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.TokenPair{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 60}, nil).
		AnyTimes()

	const racers = 2
	results := make(chan error, racers)
	start := make(chan struct{})
	for range racers {
		go func() {
			<-start
			_, err := svc.Accept(ctx, &entity.AcceptInvitationCmd{
				Token:    raw,
				Provider: entity.OAuthProviderGoogle,
				IDToken:  "tok",
			})
			results <- err
		}()
	}
	close(start)

	// Single-use is the invariant: exactly one accept may succeed. The loser
	// fails — usually "invalid" (lost the claim), possibly a create/email-race
	// error since the user is created before the claim — but never a second
	// success.
	var success, failed int
	for range racers {
		if err := <-results; err == nil {
			success++
		} else {
			failed++
		}
	}

	require.Equal(t, 1, success, "exactly one accept must succeed (single-use)")
	require.Equal(t, 1, failed, "the loser must fail, not get a second token pair")
}

// fixedCapStore is the license cache store for the accept regression: it always
// reports an active license with the given seats_purchased.
type fixedCapStore struct{ cap int64 }

func (s fixedCapStore) Get(context.Context) (*entity.License, error) {
	return &entity.License{Status: entity.LicenseStatusActive, SeatsPurchased: lo.ToPtr(s.cap)}, nil
}

func (fixedCapStore) Upsert(context.Context, *entity.License) error { return nil }

// inviteScopedSeatStore is the license.UsersStore for the accept net-zero
// regression. It counts the accepting user's active seat roles, read in-tx via
// the real store, scoped to that one user so the shared DB's population never
// swamps the cap. The user does not exist until accept creates it, so a
// not-found read contributes zero.
type inviteScopedSeatStore struct {
	store  *users.Store
	userID *uuid.UUID
}

func (s *inviteScopedSeatStore) ListActiveRoles(ctx context.Context) ([][]entity.Role, error) {
	if s.userID == nil {
		return nil, nil
	}
	u, err := s.store.GetByID(ctx, *s.userID)
	if err != nil {
		return nil, nil //nolint:nilerr // not-yet-created user occupies no seat
	}
	if u.IsBlocked() {
		return nil, nil
	}
	return [][]entity.Role{u.Roles}, nil
}

// inviteScopedPendingStore is the license.InvitationsStore for the regression.
// It reads the single invite in-tx via the real store and counts it as pending
// only while its stored status is pending. This is the load-bearing seam: if the
// guard's count runs in the SAME tx as MarkAccepted (reentrant AssignRoles), it
// sees status=accepted and returns empty (net-zero); if AssignRoles opened its
// own tx, it would still read pending and the accept would breach the cap.
type inviteScopedPendingStore struct {
	store    *userinvitations.Store
	inviteID uuid.UUID
}

func (s *inviteScopedPendingStore) ListPendingRoles(ctx context.Context) ([][]entity.Role, error) {
	inv, err := s.store.GetByID(ctx, s.inviteID)
	if err != nil {
		return nil, nil //nolint:nilerr // no invite → nothing pending
	}
	if inv.Status != entity.InvitationStatusPending {
		return nil, nil
	}
	return [][]entity.Role{inv.Roles}, nil
}

// TestAccept_NetZeroAtFullCap is the accept net-zero regression (SPEC "accept
// boundary regression"). One seat-role invite is pending under a cap of exactly
// one seat: occupancy is full. Accepting that invite must still PASS, because in
// the accept tx MarkAccepted flips the invite pending→accepted before AssignRoles
// runs the seat guard, so the guard's fresh count sees the invite gone from
// pending and the user not yet seated → occupied 0, occupied+1 = 1 <= 1.
//
// This asserts TX IDENTITY, not merely the count: the guard's ListPendingRoles
// reads the invite in-tx, so it only sees the accepted flip if AssignRoles joins
// MarkAccepted's transaction. Reorder MarkAccepted after AssignRoles, split the
// tx, or make AssignRoles open its own tx, and the guard reads the invite as
// still pending → occupied 1, occupied+1 = 2 > 1 → this accept fails. That is the
// regression this test catches.
func TestAccept_NetZeroAtFullCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	txManager := dbtx.NewTxManager(db)

	// Seat store learns the accepting user's id after creation; pending store is
	// bound to the invite id below.
	seatStore := &inviteScopedSeatStore{store: users.NewStore(db)}
	invStore := userinvitations.NewStore(db)
	pendingStore := &inviteScopedPendingStore{store: invStore}

	// Real license service, cap = 1: the single pending seat-invite fills it.
	licenseSrv := license.NewService(
		seatStore,
		pendingStore,
		nil, // audit unused
		fixedCapStore{cap: 1},
		nil,      // heartbeat client unused
		"v-test", // version
		time.Hour,
	)

	tokenIssuer := mock_invitation.NewMockTokenIssuer(ctrl)
	oauthProvider := mock_oauthprovider.NewMockOAuthProvider(ctrl)
	sender := mock_invitation.NewMockMessageSender(ctrl)

	sent := &sentEmail{}
	sender.EXPECT().
		SendAsync(gomock.Any(), entity.ProcessorTaskInvitationEmailSend, entity.NotifyTransportEmail, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ entity.NotifyTransport, target string, msg entity.NotifyMessage, _ *entity.MessageRef, _ string) error {
			sent.target = target
			sent.body = msg.Body
			return nil
		}).AnyTimes()
	oauthProvider.EXPECT().ProviderID().Return(entity.OAuthProviderGoogle).AnyTimes()

	userSrv := user.NewService(
		txManager,
		users.NewStore(db),
		useridentities.NewStore(db),
		newTestAuditPublisher(t),
		mock_user.NewMockTokenRevoker(ctrl),
		licenseSrv, // Accept → AssignRoles runs the real seat guard
		false,
	)

	svc := NewService(
		cfg,
		txManager,
		invStore,
		userSrv,
		tokenIssuer,
		oauthprovider.NewOAuthProviders(cfg, []oauthprovider.OAuthProvider{oauthProvider}),
		sender,
		licenseSrv, // Create runs the real guard too (unused here, invite made below via userSrv-free path)
	)

	// Create the pending seat-invite. Cap is 1 and Create's own guard would count
	// this invite's future seat; but at creation occupancy is 0 (no active seats,
	// no other pending), so occupied+1 = 1 <= 1 passes and the invite is stored.
	emailAddr := uniqueEmail(t)
	inv, err := svc.Create(ctx, &entity.CreateInvitationCmd{
		Actor: makeAdmin(ctx, t, svc),
		Email: emailAddr,
		Roles: []entity.Role{entity.RoleReviewer},
	})
	require.NoError(t, err)
	pendingStore.inviteID = inv.ID // the pending store now counts this invite
	raw := rawTokenFromLink(t, sent.body)

	// Bind the seat store to the user id the accept will create/resolve.
	subject := newUUID().String()
	oauthProvider.EXPECT().VerifyToken(gomock.Any(), "tok").Return(&entity.OAuthIDTokenClaims{
		Subject: subject,
		Email:   emailAddr,
		Name:    "Invited User",
	}, nil)
	tokenIssuer.EXPECT().
		IssueTokenPair(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.TokenPair{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 60}, nil)

	// Resolve the user id the accept will create, so the seat store can be bound
	// before the guard counts (the guard runs inside accept's tx, before token
	// issuance). GetOrCreate is idempotent, so pre-creating here yields the same
	// row the accept resolves.
	created, err := userSrv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, &entity.OAuthProviderUserInfo{
		ID:    subject,
		Email: emailAddr,
		Name:  "Invited User",
	}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)
	seatStore.userID = lo.ToPtr(created.ID)

	// The accept must PASS at full cap — net-zero. A failure here means the tx
	// identity broke (guard no longer shares MarkAccepted's tx).
	pair, err := svc.Accept(ctx, &entity.AcceptInvitationCmd{
		Token:    raw,
		Provider: entity.OAuthProviderGoogle,
		IDToken:  "tok",
		ClientIP: "127.0.0.1",
	})
	require.NoError(t, err, "accept at full cap must pass (net-zero): the invite's own seat is released as it is accepted")
	require.Equal(t, "access", pair.AccessToken)

	// The user ended up seated with the invite's role.
	got, err := userSrv.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Contains(t, got.Roles, entity.RoleReviewer)
}
