package resources

import (
	"context"
	"testing"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	resourcesstore "github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// Every resource-catalog mutation is recorded with its actor. Create is
// get-or-create, so asking for an existing name inserts nothing and records
// nothing; the idempotent repeats of archive / unarchive likewise.
func TestResourceCatalog_PublishesAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	rec := &recordingAuditPublisher{}
	service := NewService(dbtx.NewTxManager(db), resourcesstore.NewStore(db), rec)
	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}
	name := "audit-" + xuuid.NewString()

	resource, err := service.CreateResource(ctx, &entity.CreateResourceCmd{Name: name, Description: "before", Actor: actor})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := table.Resources.DELETE().
			WHERE(table.Resources.ID.EQ(postgres.UUID(resource.ID))).
			ExecContext(context.Background(), db)
		require.NoError(t, err)
	})

	again, err := service.CreateResource(ctx, &entity.CreateResourceCmd{Name: name, Actor: actor})
	require.NoError(t, err)
	require.Equal(t, resource.ID, again.ID, "get-or-create returns the existing row")

	_, err = service.UpdateResource(ctx, &entity.UpdateResourceCmd{
		ID:          resource.ID,
		Description: lo.ToPtr("after"),
		Actor:       actor,
	})
	require.NoError(t, err)

	require.NoError(t, service.ArchiveResource(ctx, actor, resource.ID))
	require.NoError(t, service.ArchiveResource(ctx, actor, resource.ID))
	require.NoError(t, service.UnarchiveResource(ctx, actor, resource.ID))
	require.NoError(t, service.UnarchiveResource(ctx, actor, resource.ID))
	require.NoError(t, service.ArchiveResource(ctx, actor, uuid.New()), "an unknown id stays a no-op success")

	published := rec.published()
	require.Len(t, published, 4, "create, update, archive, unarchive -- and nothing for the repeats")

	created, ok := published[0].(audit.ResourceCreated)
	require.True(t, ok, "got %T", published[0])
	require.Equal(t, actor, created.Actor)
	require.Equal(t, resource.ID, created.Resource.ID)

	updated, ok := published[1].(audit.ResourceUpdated)
	require.True(t, ok, "got %T", published[1])
	require.Equal(t, actor, updated.Actor)
	require.Equal(t, []entity.AuditFieldChange{{Field: "description", Old: "before", New: "after"}}, updated.Changes)

	archived, ok := published[2].(audit.ResourceArchived)
	require.True(t, ok, "got %T", published[2])
	require.Equal(t, resource.ID, archived.Resource.ID)
	require.Equal(t, name, archived.Resource.Name)

	unarchived, ok := published[3].(audit.ResourceUnarchived)
	require.True(t, ok, "got %T", published[3])
	require.Equal(t, resource.ID, unarchived.Resource.ID)

	// The state transitions themselves still happened.
	got, err := service.GetResourceByID(ctx, resource.ID)
	require.NoError(t, err)
	require.Equal(t, entity.ResourceStatusActive, got.Status)
}
