# Маппинг данных

## Entity -> Model (toDBEntity)

Конвертация из бизнес-логики entity в database model.

```go
package maintenances

import (
    "github.com/google/uuid"
    "github.com/samber/lo"

    "github.com/ruko1202/maintmode/internal/entity"
    "github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
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

    // Обработка nullable полей
    if m.ActualPeriod != nil {
        actualPeriod := xtime.ToPgRange(lo.FromPtr(m.ActualPeriod))
        maint.ActualPeriod = &actualPeriod
    }

    return maint
}
```

## Model -> Entity (fromDBEntity)

Конвертация из database model в бизнес-логику entity.

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

    // Обработка nullable полей
    if m.ActualPeriod != nil {
        actualPeriod := xtime.FromPgRange(lo.FromPtr(m.ActualPeriod))
        maint.ActualPeriod = &actualPeriod
    }

    return maint
}
```

## Работа с enum типами

```go
// Entity использует типизированные enum
type MaintenanceStatus string

const (
    StatusDraft     MaintenanceStatus = "draft"
    StatusScheduled MaintenanceStatus = "scheduled"
    StatusActive    MaintenanceStatus = "active"
    StatusCompleted MaintenanceStatus = "completed"
)

// Конвертация в model
Status: string(m.Status)

// Конвертация из model
Status: entity.MaintenanceStatus(m.Status)
```

## Работа с вложенными структурами

```go
// Для составных ключей или связанных сущностей
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

## Обработка nullable полей

```go
import "github.com/samber/lo"

// Конвертация в nullable pointer
CanceledReasonCode: lo.ToPtr(string(m.CancelReason))

// Конвертация из nullable pointer
CancelReason: entity.MaintenanceCancelReason(lo.FromPtr(m.CanceledReasonCode))

// Условная конвертация
if m.ActualPeriod != nil {
    actualPeriod := xtime.FromPgRange(lo.FromPtr(m.ActualPeriod))
    maint.ActualPeriod = &actualPeriod
}
```

## Конвертация временных типов

```go
// Postgres range types
import "github.com/ruko1202/maintmode/internal/utils/xtime"

// Entity -> Model (tstzrange)
PlannedPeriod: xtime.ToPgRange(m.PlannedPeriod)

// Model -> Entity
PlannedPeriod: xtime.FromPgRange(m.PlannedPeriod)
```
