package user

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/license"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// seatCountBarrier synchronizes the two racing transactions right before the
// occupancy count. It releases as soon as both racers arrive — the racy path,
// where both count concurrently and (without the lock) both observe room. The
// timeout is the escape hatch for the serialized path: with the advisory lock,
// the second racer is parked behind the lock and never reaches the barrier while
// the first holds it, so the barrier releases on timeout and the second racer
// then counts the first's committed seat.
type seatCountBarrier struct {
	mu      sync.Mutex
	arrived int
	full    chan struct{}
	timeout time.Duration
}

func newSeatCountBarrier(timeout time.Duration) *seatCountBarrier {
	return &seatCountBarrier{full: make(chan struct{}), timeout: timeout}
}

func (b *seatCountBarrier) await() {
	b.mu.Lock()
	b.arrived++
	if b.arrived == 2 {
		close(b.full)
	}
	b.mu.Unlock()

	select {
	case <-b.full:
	case <-time.After(b.timeout):
	}
}

// pairScopedSeatStore is the license.UsersStore for the seat race. The shared
// dev DB carries many active seat-users, so a global ListActiveRoles would swamp
// any small cap and the race could never surface. This wrapper scopes the count
// to the two racing users, read with real GetByID inside the caller's tx (so
// each racer sees its own uncommitted snapshot) — keeping the guard's count-in-tx
// mechanics intact — and awaits the barrier so both racers count concurrently on
// the unlocked path.
type pairScopedSeatStore struct {
	store   *users.Store
	pair    [2]uuid.UUID
	barrier *seatCountBarrier
}

func (s *pairScopedSeatStore) ListActiveRoles(ctx context.Context) ([][]entity.Role, error) {
	s.barrier.await()

	var roles [][]entity.Role
	for _, id := range s.pair {
		u, err := s.store.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get pair user: %w", err)
		}
		if !u.IsBlocked() {
			roles = append(roles, u.Roles)
		}
	}
	return roles, nil
}

// emptyPendingStore is the license.InvitationsStore for the seat race: no pending
// invites, so occupancy is entirely the two racing users' active seats.
type emptyPendingStore struct{}

func (emptyPendingStore) ListPendingRoles(context.Context) ([][]entity.Role, error) {
	return nil, nil
}

// fixedCapStore is the license cache store: it always reports an active license
// with the given seats_purchased, so the guard enforces exactly that cap.
type fixedCapStore struct{ cap int64 }

func (s fixedCapStore) Get(context.Context) (*entity.License, error) {
	return &entity.License{Status: entity.LicenseStatusActive, SeatsPurchased: lo.ToPtr(s.cap)}, nil
}

func (fixedCapStore) Upsert(context.Context, *entity.License) error { return nil }

// TestAssignRoles_SeatCapRace reproduces the write-skew the seat advisory lock
// prevents: two users are concurrently lifted guest→editor with exactly one seat
// free. Each transaction counts occupancy before persisting its own grant. Under
// READ COMMITTED without the lock, both count zero occupied seats, both pass
// occupied+1 <= cap, and both commit — oversubscribing the license. The advisory
// lock must serialize the count-then-write decision so exactly one grant succeeds
// and the other gets ErrSeatsLimitExceeded.
//
// Mutate-to-prove (B2): removing dbtx.AdvisoryXactLock from
// license.Service.EnsureSeatAvailable makes this test FAIL — both racers pass and
// two seats are granted under a cap of one. That failure is the proof the lock,
// not luck, is what serializes the grant. Verified once during T8, then restored.
func TestAssignRoles_SeatCapRace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Setup goes through a Noop-guard service so fixtures are never capped.
	setupSrv := initService(t)
	userA := makeUser(ctx, t, setupSrv) // guest
	userB := makeUser(ctx, t, setupSrv) // guest
	actor := makeUser(ctx, t, setupSrv, entity.RoleAdmin)

	// Cap = 1: both users start as guests (0 seats occupied), so exactly one of
	// the two concurrent guest→editor grants fits.
	licenseSrv := license.NewService(
		&pairScopedSeatStore{
			store:   users.NewStore(db),
			pair:    [2]uuid.UUID{userA.ID, userB.ID},
			barrier: newSeatCountBarrier(1500 * time.Millisecond),
		},
		emptyPendingStore{},
		nil, // audit: unused by EnsureSeatAvailable
		fixedCapStore{cap: 1},
		nil,      // heartbeat client: unused by EnsureSeatAvailable
		"v-test", // version
		time.Hour,
	)

	raceSrv := NewService(
		dbtx.NewTxManager(db),
		users.NewStore(db),
		useridentities.NewStore(db),
		newTestAuditPublisher(t),
		&fakeTokenRevoker{},
		licenseSrv,
		false,
	)

	const racers = 2
	results := make(chan error, racers)
	start := make(chan struct{})
	assign := func(target *entity.User) {
		<-start
		_, err := raceSrv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			UserID: target.ID,
			Roles:  []entity.Role{entity.RoleEditor},
		})
		results <- err
	}
	go assign(userA)
	go assign(userB)
	close(start)

	var seatErrs, granted int
	for range racers {
		err := <-results
		switch {
		case err == nil:
			granted++
		case errors.Is(err, apperr.ErrSeatsLimitExceeded):
			seatErrs++
		default:
			t.Errorf("unexpected assign error: %v", err)
		}
	}

	// The load-bearing assertion: exactly one grant wins the last seat.
	require.Equal(t, 1, granted, "exactly one guest→seat grant must succeed")
	require.Equal(t, 1, seatErrs, "exactly one grant must be rejected with ErrSeatsLimitExceeded")

	// And the DB agrees: exactly one of the pair holds a seat role.
	var seated int
	for _, id := range []uuid.UUID{userA.ID, userB.ID} {
		u, err := setupSrv.GetByID(ctx, id)
		require.NoError(t, err)
		if entity.RoleOccupiesSeat(entity.HighestRole(u.Roles)) {
			seated++
		}
	}
	require.Equal(t, 1, seated, "exactly one user must end up seated (no oversell)")
}
