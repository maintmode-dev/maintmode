package invitation

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// fakeDanceHandles maps handles to invitation ids, and forgets a handle once it
// is redeemed — the single-use behavior the real store gets from GETDEL.
type fakeDanceHandles struct {
	ids map[string]uuid.UUID
	err error
}

func newFakeDanceHandles() *fakeDanceHandles {
	return &fakeDanceHandles{ids: map[string]uuid.UUID{}}
}

func (f *fakeDanceHandles) put(handle string, id uuid.UUID) { f.ids[handle] = id }

func (f *fakeDanceHandles) PutInvitationHandle(_ context.Context, handle string, id uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.ids[handle] = id

	return nil
}

func (f *fakeDanceHandles) ConsumeInvitationHandle(_ context.Context, handle string) (*uuid.UUID, error) {
	if f.err != nil {
		return nil, f.err
	}

	id, ok := f.ids[handle]
	if !ok {
		return nil, nil
	}
	delete(f.ids, handle)

	return &id, nil
}

// armHandles wires a fake handle store and returns a handle already pointing at
// inv.
func armHandles(t *testing.T, svc *Service, inv *entity.Invitation) string {
	t.Helper()

	handles := newFakeDanceHandles()
	svc.WithDanceHandles(handles)

	handle := uuid.NewString()
	handles.put(handle, inv.ID)

	return handle
}

func TestResolveForIdentityGuards(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("matching email resolves to the invitation and its roles", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		email := uniqueEmail(t)
		inv := mustCreate(ctx, t, svc, email, entity.RoleReviewer)
		handle := armHandles(t, svc, inv)

		got, err := svc.ResolveForIdentity(ctx, handle, &entity.OAuthIDTokenClaims{Email: email})
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, inv.ID, got.ID)
		assert.Equal(t, []entity.Role{entity.RoleReviewer}, got.Roles)
	})

	// The anti-takeover guard. Without it, whoever holds the link can accept it
	// with any account they control.
	t.Run("a different email is refused as a mismatch", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		inv := mustCreate(ctx, t, svc, uniqueEmail(t))
		handle := armHandles(t, svc, inv)

		_, err := svc.ResolveForIdentity(ctx, handle, &entity.OAuthIDTokenClaims{Email: uniqueEmail(t)})
		require.ErrorIs(t, err, apperr.ErrEmailMismatch)
	})

	// Case-insensitivity is the documented behavior of the shared guard; an
	// invitee whose provider reports a differently-cased address is the same
	// person, not an attacker.
	t.Run("case differences still match", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		email := uniqueEmail(t)
		inv := mustCreate(ctx, t, svc, email)
		handle := armHandles(t, svc, inv)

		got, err := svc.ResolveForIdentity(ctx, handle, &entity.OAuthIDTokenClaims{
			Email: strings.ToUpper(email),
		})
		require.NoError(t, err)
		assert.Equal(t, inv.ID, got.ID)
	})

	t.Run("an unknown handle is invalid", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		svc.WithDanceHandles(newFakeDanceHandles())

		_, err := svc.ResolveForIdentity(ctx, uuid.NewString(), &entity.OAuthIDTokenClaims{
			Email: uniqueEmail(t),
		})
		require.ErrorIs(t, err, apperr.ErrInvalidInvitation)
	})

	// Expiry is DERIVED, not stored: an expired invitation keeps status=pending
	// with expires_at in the past, and only EffectiveStatus resolves it. A test
	// that checks the raw column would pass while an invitation emailed a month
	// ago still created accounts forever. The email deliberately MATCHES so this
	// pins the status branch rather than the mismatch branch.
	t.Run("an expired invitation is invalid", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		email := uniqueEmail(t)

		expired, err := svc.store.Create(ctx, &entity.Invitation{
			Email:       email,
			Roles:       []entity.Role{entity.RoleEditor},
			TokenHash:   xhash.HashSha256([]byte(xuuid.NewString())),
			Status:      entity.InvitationStatusPending,
			ExpiresAt:   xtime.UTCNow().Add(-time.Hour),
			SentAt:      xtime.UTCNow().Add(-2 * time.Hour),
			InvitedByID: makeAdmin(ctx, t, svc).ID,
		})
		require.NoError(t, err)
		require.Equal(t, entity.InvitationStatusPending, expired.Status,
			"the stored column must still say pending, or this tests nothing")

		handles := newFakeDanceHandles()
		svc.WithDanceHandles(handles)
		handle := uuid.NewString()
		handles.put(handle, expired.ID)

		_, err = svc.ResolveForIdentity(ctx, handle, &entity.OAuthIDTokenClaims{Email: email})
		require.ErrorIs(t, err, apperr.ErrInvalidInvitation)
	})

	t.Run("a revoked invitation is invalid", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		email := uniqueEmail(t)
		inv := mustCreate(ctx, t, svc, email)
		require.NoError(t, svc.Revoke(ctx, &entity.RevokeInvitationCmd{
			Actor: makeAdmin(ctx, t, svc), ID: inv.ID,
		}))
		handle := armHandles(t, svc, inv)

		// Note the email MATCHES: the refusal is about status, and it must not
		// be reported as a mismatch, which would tell the caller the address was
		// right.
		_, err := svc.ResolveForIdentity(ctx, handle, &entity.OAuthIDTokenClaims{Email: email})
		require.ErrorIs(t, err, apperr.ErrInvalidInvitation)
	})

	// Fail closed: a store that cannot answer must never read as "no invitation
	// and carry on", but it must also never be mistaken for a valid one.
	t.Run("a store failure propagates rather than resolving", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		handles := newFakeDanceHandles()
		handles.err = errors.New("valkey is down")
		svc.WithDanceHandles(handles)

		_, err := svc.ResolveForIdentity(ctx, "h", &entity.OAuthIDTokenClaims{Email: uniqueEmail(t)})
		require.Error(t, err)

		// NOT ErrInvalidInvitation: that is the answer for a handle that named
		// nothing, and collapsing an unreachable store into it would report a
		// working invitation as invalid — hiding an outage as a user error and
		// costing the metric that makes it visible. Asserting NotErrorIs on the
		// mismatch instead would be a tautology; the fake never returns one.
		assert.NotErrorIs(t, err, apperr.ErrInvalidInvitation)
	})

	// An instance with no dance configured has no handle store at all. It must
	// refuse rather than panic on a public route.
	t.Run("no handle store refuses instead of panicking", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)

		_, err := svc.ResolveForIdentity(ctx, "h", &entity.OAuthIDTokenClaims{Email: uniqueEmail(t)})
		require.ErrorIs(t, err, apperr.ErrInvalidInvitation)
	})
}

func TestClaimForUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// The ordering guard: MarkAccepted must run BEFORE AssignRoles.
	//
	// This is load-bearing and invisible to the compiler. The real seats guard
	// counts live PENDING invitations as occupied, and it runs inside the claim
	// transaction, so it sees this transaction's own uncommitted writes. Marking
	// first releases the invitee's reservation before the count, and the guard's
	// occupied+1 re-adds the same person -- net zero. Assigning first leaves the
	// invitation pending while the guard counts, so the invitee is counted twice
	// and, at exactly the cap, refused their own reserved seat.
	//
	// The assertion has to observe what the guard SEES, not what it returns: the
	// invitation's status at the moment the guard runs. Asserting the rollback
	// instead proves nothing -- the transaction rolls back under either order.
	t.Run("the invitation is already spent when the seats guard counts", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initService(t)
		inv := mustCreate(ctx, t, svc, uniqueEmail(t), entity.RoleEditor)
		user := makeAdmin(ctx, t, svc)

		// Read the invitation's status from inside the guard, on the claim's own
		// transaction -- exactly where the real guard's ListPendingRoles reads it.
		var statusWhenGuardRan entity.InvitationStatus
		mocks.seatGuard.onCall = func(ctx context.Context) {
			stored, err := svc.store.GetByID(ctx, inv.ID)
			require.NoError(t, err)
			statusWhenGuardRan = stored.Status
		}

		require.NoError(t, svc.ClaimForUser(ctx, &entity.ResolvedInvitation{
			ID: inv.ID, Roles: inv.Roles,
		}, user.ID))

		require.Positive(t, mocks.seatGuard.called, "the seats guard must have run")
		assert.Equal(t, entity.InvitationStatusAccepted, statusWhenGuardRan,
			"MarkAccepted must precede AssignRoles: the guard has to count a world "+
				"where this invitation no longer holds a pending seat, or it charges "+
				"the invitee twice and refuses them their own reserved seat")
	})
}

// TestNilUUIDHandleNeverResolves pins the assumption PrepareHandle rests on.
//
// A token that names no invitation still gets a stored handle -- pointing at
// uuid.Nil -- so that /start does the same work either way and cannot be used
// to tell live tokens from dead ones before authenticating. That is only safe
// while uuid.Nil names no real invitation. Invitation ids are generated, so it
// cannot; this test is what would notice if that ever stopped being true.
func TestNilUUIDHandleNeverResolves(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _ := initService(t)

	handles := newFakeDanceHandles()
	svc.WithDanceHandles(handles)
	handle := uuid.NewString()
	handles.put(handle, uuid.Nil)

	_, err := svc.ResolveForIdentity(ctx, handle, &entity.OAuthIDTokenClaims{Email: uniqueEmail(t)})
	require.ErrorIs(t, err, apperr.ErrInvalidInvitation)

	_, getErr := svc.store.GetByID(ctx, uuid.Nil)
	assert.ErrorIs(t, getErr, apperr.ErrInvitationNotFound,
		"no invitation row may be addressable by uuid.Nil")
}

// TestClaimForUserConcurrent proves the property ClaimForUser's doc comment
// actually claims: of two CONCURRENT claims of one invitation, exactly one
// wins. The sequential test above cannot show that -- it only proves the second
// call sees an already-accepted row.
//
// Deterministic in the way the repo's store tests are: a shared channel
// releases both racers at once, and the round count absorbs the fact that a
// race is probabilistic. Without the release channel, goroutine startup skew
// serializes them and the test passes against an implementation with no gate at
// all.
func TestClaimForUserConcurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const rounds = 10

	for range rounds {
		svc, _ := initService(t)
		inv := mustCreate(ctx, t, svc, uniqueEmail(t), entity.RoleEditor)
		resolved := &entity.ResolvedInvitation{ID: inv.ID, Roles: inv.Roles}

		userA := makeAdmin(ctx, t, svc)
		userB := makeAdmin(ctx, t, svc)

		start := make(chan struct{})
		results := make(chan error, 2)

		var wg sync.WaitGroup
		for _, userID := range []uuid.UUID{userA.ID, userB.ID} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				results <- svc.ClaimForUser(ctx, resolved, userID)
			}()
		}

		close(start)
		wg.Wait()
		close(results)

		var won int
		for err := range results {
			if err == nil {
				won++
			}
		}

		require.Equal(t, 1, won,
			"exactly one concurrent claim may spend an invitation; one link must "+
				"never onboard two accounts")
	}
}
