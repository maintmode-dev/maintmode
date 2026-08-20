# Data mapping

## Entity -> Model (toDBEntity)

Conversion from a business-logic entity into a database model.

```go
package maintenances

import (
    "github.com/google/uuid"
    "github.com/samber/lo"

    "github.com/ruko1202/maintmode/internal/entity"
    "github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
    "github.com/ruko1202/maintmode/internal/utils/xtime"
)

func toDBMaintenance(m *entity.Maintenance) *model.Maintenances {
    maint := &model.Maintenances{
        ID:                    m.ID,
        Title:                 m.Title,
        Description:           m.Description,
        PlannedPeriod:         xtime.ToPgRange(m.PlannedPeriod),
        Scope:                 string(m.Scope),
        Impact:                string(m.Impact),
        Status:                string(m.Status),
        CanceledReasonCode:    lo.ToPtr(string(m.CancelReason)),
        CanceledReasonComment: lo.ToPtr(m.CancelReasonComment),
        CreatedAt:             m.CreatedAt,
        UpdatedAt:             m.UpdatedAt,
    }

    // Handling nullable fields
    if m.ActualPeriod != nil {
        actualPeriod := xtime.ToPgRange(lo.FromPtr(m.ActualPeriod))
        maint.ActualPeriod = &actualPeriod
    }

    return maint
}
```

## Model -> Entity (fromDBEntity)

Conversion from a database model into a business-logic entity.

```go
func fromDBMaintenance(m *model.Maintenances) *entity.Maintenance {
    maint := &entity.Maintenance{
        ID:                  m.ID,
        Title:               m.Title,
        Description:         m.Description,
        PlannedPeriod:       xtime.FromPgRange(m.PlannedPeriod),
        Scope:               entity.MaintenanceScope(m.Scope),
        Impact:              entity.MaintenanceImpact(m.Impact),
        Status:              entity.MaintenanceStatus(m.Status),
        CancelReason:        entity.MaintenanceCancelReason(lo.FromPtr(m.CanceledReasonCode)),
        CancelReasonComment: lo.FromPtr(m.CanceledReasonComment),
        CreatedAt:           m.CreatedAt,
        UpdatedAt:           m.UpdatedAt,
    }

    // Handling nullable fields
    if m.ActualPeriod != nil {
        actualPeriod := xtime.FromPgRange(lo.FromPtr(m.ActualPeriod))
        maint.ActualPeriod = &actualPeriod
    }

    return maint
}
```

## Working with enum types

```go
// The entity uses typed enums
type MaintenanceStatus string

const (
    StatusDraft     MaintenanceStatus = "draft"
    StatusScheduled MaintenanceStatus = "scheduled"
    StatusActive    MaintenanceStatus = "active"
    StatusCompleted MaintenanceStatus = "completed"
)

// Conversion into the model
Status: string(m.Status)

// Conversion from the model
Status: entity.MaintenanceStatus(m.Status)
```

## Working with nested structures

```go
// For composite keys or related entities
type MaintenanceResource struct {
    MaintenanceID uuid.UUID
    ResourceID    uuid.UUID
    ResourceType  string
}

func toDBMaintenanceResource(maintID uuid.UUID, r *entity.Resource) *model.MaintenanceResources {
    return &model.MaintenanceResources{
        MaintenanceID: maintID,
        ResourceID:    r.ID,
        ResourceType:  string(r.Type),
    }
}
```

## Handling nullable fields

```go
import "github.com/samber/lo"

// Conversion into a nullable pointer
CanceledReasonCode: lo.ToPtr(string(m.CancelReason))

// Conversion from a nullable pointer
CancelReason: entity.MaintenanceCancelReason(lo.FromPtr(m.CanceledReasonCode))

// Conditional conversion
if m.ActualPeriod != nil {
    actualPeriod := xtime.FromPgRange(lo.FromPtr(m.ActualPeriod))
    maint.ActualPeriod = &actualPeriod
}
```

## Converting time types

```go
// Postgres range types
import "github.com/ruko1202/maintmode/internal/utils/xtime"

// Entity -> Model (tstzrange)
PlannedPeriod: xtime.ToPgRange(m.PlannedPeriod)

// Model -> Entity
PlannedPeriod: xtime.FromPgRange(m.PlannedPeriod)
```
