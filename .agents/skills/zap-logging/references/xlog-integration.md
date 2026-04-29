# xlog Integration

MaintMode already depends on `github.com/ruko1202/xlog`. Do not create a local `internal/utils/xlog` package.

## Store Usage

```go
import (
    "github.com/ruko1202/xlog"
    "github.com/ruko1202/xlog/xfield"
)

func (s *Store) Get(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
    ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.Get")
    defer span.End()

    stmt := table.Maintenances.
        SELECT(table.Maintenances.AllColumns).
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(maintID)))

    maint := new(model.Maintenances)
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), maint)
    if err != nil {
        xlog.Error(ctx, "failed to get maintenance",
            xfield.String("maint_id", maintID.String()),
            xfield.Error(err),
        )
        return nil, err
    }

    return fromDBMaintenance(maint), nil
}
```
