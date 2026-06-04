package invitation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
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
