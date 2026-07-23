package user

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// countBarrier synchronizes the two racing transactions right before the admin
// count. It releases as soon as both racers arrive — that is the racy path,
// where both transactions count concurrently. The timeout is only the escape
// hatch for the serialized path, where the second racer is parked behind the
// guard's lock and never reaches the barrier while the first one holds it.
type countBarrier struct {
	mu      sync.Mutex
	arrived int
	full    chan struct{}
	timeout time.Duration
}

func newCountBarrier(timeout time.Duration) *countBarrier {
	return &countBarrier{full: make(chan struct{}), timeout: timeout}
}

func (b *countBarrier) await() {
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

// pairScopedCountStore wraps the real users store for the last-admin race
// test. The shared dev DB carries many unrelated admins, so the guard's global
// CountActiveAdmins would never drop to the lockout threshold and the race
// could not surface through ErrLastAdmin. The wrapper keeps the guard's
// mechanics intact — the count still runs inside the caller's transaction via
// the ctx executor, so each racer observes its own snapshot — but scopes the
// count to the two test admins, counted with real GetByID reads.
type pairScopedCountStore struct {
	UsersStore
	pair    [2]uuid.UUID
	barrier *countBarrier
}

func (s *pairScopedCountStore) CountActiveAdmins(ctx context.Context) (int64, error) {
	s.barrier.await()

	var count int64
	for _, id := range s.pair {
		u, err := s.GetByID(ctx, id)
		if err != nil {
			return 0, fmt.Errorf("get pair admin: %w", err)
		}
		if u.IsActiveAdmin() {
			count++
		}
	}

	return count, nil
}

// TestRevokeRole_LastAdminRace reproduces the write-skew in the last-admin
// guard: two admins concurrently revoke the admin role from each other. Each
// transaction FOR-UPDATE-locks only the *other* admin's row, so under READ
// COMMITTED both count two active admins, both pass the guard and both commit,
// leaving zero active admins. The guard must serialize the count-then-write
// decision so that exactly one revoke fails with ErrLastAdmin and exactly one
// of the pair stays an active admin.
func TestRevokeRole_LastAdminRace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Setup goes through a plain service: user creation (the bootstrap path)
	// also consults CountActiveAdmins and must not hit the barriered wrapper.
	setupSrv := initService(t)
	adminA := makeUser(ctx, t, setupSrv, entity.RoleAdmin)
	adminB := makeUser(ctx, t, setupSrv, entity.RoleAdmin)

	raceSrv := NewService(
		dbtx.NewTxManager(db),
		&pairScopedCountStore{
			UsersStore: users.NewStore(db),
			pair:       [2]uuid.UUID{adminA.ID, adminB.ID},
			barrier:    newCountBarrier(1500 * time.Millisecond),
		},
		useridentities.NewStore(db),
		newTestAuditPublisher(t),
		&fakeTokenRevoker{},
		false,
	)

	const racers = 2
	results := make(chan error, racers)
	start := make(chan struct{})
	revoke := func(actor, target *entity.User) {
		<-start
		results <- raceSrv.RevokeRole(ctx, &entity.RevokeRoleCmd{
			Actor:  actor,
			UserID: target.ID,
			Role:   entity.RoleAdmin,
		})
	}
	go revoke(adminA, adminB)
	go revoke(adminB, adminA)
	close(start)

	var lastAdminErrs int
	for range racers {
		err := <-results
		switch {
		case err == nil:
		case errors.Is(err, apperr.ErrLastAdmin):
			lastAdminErrs++
		default:
			t.Errorf("unexpected revoke error: %v", err)
		}
	}

	var activeAdmins int
	for _, id := range []uuid.UUID{adminA.ID, adminB.ID} {
		u, err := setupSrv.GetByID(ctx, id)
		require.NoError(t, err)
		if u.IsActiveAdmin() {
			activeAdmins++
		}
	}

	require.Equal(t, 1, activeAdmins, "exactly one of the two admins must stay active")
	require.Equal(t, 1, lastAdminErrs, "exactly one revoke must be rejected with ErrLastAdmin")
}
