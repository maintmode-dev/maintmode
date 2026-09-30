package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Provisioning waits for a lock another replica holds, and then works from the
// state that replica left: a row written meanwhile is taken over, not inserted
// a second time into UNIQUE (kind, name).
//
// The race is reproduced, not hoped for: the test holds the lock itself, waits
// until Provision is queued on it, commits the competing row and only then
// lets go. Without the lock Provision never queues, and the test fails on the
// wait -- or, had it raced ahead, on the competing insert.
//
// Not parallel: it provisions (see provision_notify_test.go).
func TestProvision_WaitsForAnotherReplicasLock(t *testing.T) {
	ctx := context.Background()
	adminSvc, kinds, _ := initService(t)
	svc, _ := newServiceDeclaring(t, kinds, nil, nil, config.NotifyTransportEntries{
		kinds.slack: declaredTransport(nil, map[string]string{"bot_token": "cfg-token"}),
	})
	key := int64(dbtx.AdvisoryLockKeyLoginProvisioning)

	held, err := db.BeginTxx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Rollback() })
	_, err = held.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, key)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- svc.Provision(ctx) }()

	require.Eventually(t, func() bool {
		var waiting int
		err := db.GetContext(ctx, &waiting,
			`SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND objid = $1 AND NOT granted`, key)
		return err == nil && waiting > 0
	}, 5*time.Second, 10*time.Millisecond, "Provision must queue on the lock")

	_, err = adminSvc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind: kinds.notify, Name: kinds.slack, Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{}`),
		Secrets: json.RawMessage(`{"bot_token":"admin-typed"}`),
		Actor:   testActor(),
	})
	require.NoError(t, err, "the queued Provision has written nothing yet")
	before, _ := readRow(ctx, t, kinds.notify, kinds.slack)

	require.NoError(t, held.Commit())
	require.NoError(t, <-done)

	after, _ := readRow(ctx, t, kinds.notify, kinds.slack)
	require.Equal(t, before.ID, after.ID, "the row written meanwhile is taken over")
	require.True(t, after.Provisioned)
	require.JSONEq(t, `{}`, after.Secrets)
}
