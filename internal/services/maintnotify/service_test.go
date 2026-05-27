package maintnotify

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/maintnotify/router"
)

// newNotifierForTest builds a Service with the given routes and a nil
// sender. Tests that don't reach dispatch (validation, no-op routes,
// idempotency-key) can use it; tests that need actual delivery would
// require either an integration setup or a refactor of Service to
// accept a Sender interface.
func newNotifierForTest(t *testing.T) *Service {
	t.Helper()
	cfg := &config.AppConfig{
		App:         config.App{FrontendURL: "https://maintmode.test"},
		MaintNotify: config.MaintNotifyConfig{},
	}
	n, err := NewNotifier(cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, n)
	return n
}

func TestNotifyMaintLifecycle_RejectsStepKind(t *testing.T) {
	t.Parallel()
	n := newNotifierForTest(t)
	err := n.NotifyMaintLifecycle(context.Background(), entity.NotifyEventStepStarted,
		&entity.Maintenance{ID: uuid.New()})
	require.Error(t, err)
}

func TestNotifyStepLifecycle_RejectsMaintKind(t *testing.T) {
	t.Parallel()
	n := newNotifierForTest(t)
	err := n.NotifyStepLifecycle(context.Background(), entity.NotifyEventMaintStarted,
		&entity.Maintenance{ID: uuid.New()}, &entity.MaintenanceStep{ID: uuid.New()})
	require.Error(t, err)
}

// TestNotifyMaintLifecycle_NoRoutes_NoOp: with empty routes, dispatch
// short-circuits on len(routes)==0 before touching the sender, so a
// nil sender is safe.
func TestNotifyMaintLifecycle_NoRoutes_NoOp(t *testing.T) {
	t.Parallel()
	n := newNotifierForTest(t)
	err := n.NotifyMaintLifecycle(context.Background(), entity.NotifyEventMaintStarted,
		&entity.Maintenance{ID: uuid.New(), Title: "m"})
	require.NoError(t, err)
}

func TestNotifyStepLifecycle_NoRoutes_NoOp(t *testing.T) {
	t.Parallel()
	n := newNotifierForTest(t)
	err := n.NotifyStepLifecycle(context.Background(), entity.NotifyEventStepStarted,
		&entity.Maintenance{ID: uuid.New(), Title: "m"},
		&entity.MaintenanceStep{ID: uuid.New(), Order: 1, Description: "d"})
	require.NoError(t, err)
}

func TestIdempotencyKey_StableForSameEventAndRoute(t *testing.T) {
	t.Parallel()
	evt := entity.NotifyEvent{
		Kind:    entity.NotifyEventMaintStarted,
		MaintID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	}
	route := router.Route{Transport: entity.MessengerSlack, Target: "#x"}

	require.Equal(t, idempotencyKey(evt, route), idempotencyKey(evt, route),
		"same (event, maint, route) must produce identical key for goque dedupe")
}

func TestIdempotencyKey_DifferentForDifferentRoutes(t *testing.T) {
	t.Parallel()
	evt := entity.NotifyEvent{Kind: entity.NotifyEventMaintStarted, MaintID: uuid.New()}
	slack := router.Route{Transport: entity.MessengerSlack, Target: "#x"}
	telegram := router.Route{Transport: entity.MessengerTelegram, Target: "42"}

	require.NotEqual(t, idempotencyKey(evt, slack), idempotencyKey(evt, telegram),
		"different routes must produce different keys so fan-out doesn't dedupe")
}

func TestIdempotencyKey_DifferentForDifferentEvents(t *testing.T) {
	t.Parallel()
	maintID := uuid.New()
	route := router.Route{Transport: entity.MessengerSlack, Target: "#x"}

	started := idempotencyKey(entity.NotifyEvent{Kind: entity.NotifyEventMaintStarted, MaintID: maintID}, route)
	completed := idempotencyKey(entity.NotifyEvent{Kind: entity.NotifyEventMaintCompleted, MaintID: maintID}, route)

	require.NotEqual(t, started, completed,
		"different event kinds for the same maint must produce different keys")
}
