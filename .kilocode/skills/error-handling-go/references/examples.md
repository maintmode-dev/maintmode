# Complete Error Handling Examples

## Example 1: Complete Service Implementation

### Service with Error Handling

```go
package maintenance

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/ruko1202/xlog"
    "go.uber.org/zap"

    "github.com/ruko1202/maintmode/internal/apperr"
    "github.com/ruko1202/maintmode/internal/entity"
)

type Service struct {
    repo   Repository
    logger *zap.Logger
}

func NewService(repo Repository, logger *zap.Logger) *Service {
    return &Service{
        repo:   repo,
        logger: logger,
    }
}

func (s *Service) GetMaintenance(ctx context.Context, id uuid.UUID) (*entity.Maintenance, error) {
    ctx = xlog.WithOperation(ctx, "service.Maintenance.Get")

    // Get from repository
    maint, err := s.repo.Get(ctx, id)
    if err != nil {
        // Expected error - just pass through
        if errors.Is(err, apperr.ErrMaintNotFound) {
            xlog.Warn(ctx, "maintenance not found", zap.String("id", id.String()))
            return nil, err
        }

        // Unexpected error - wrap with context
        xlog.Error(ctx, "failed to get maintenance", zap.Error(err), zap.String("id", id.String()))
        return nil, fmt.Errorf("get maintenance: %w", err)
    }

    return maint, nil
}

func (s *Service) CreateMaintenance(ctx context.Context, req *CreateMaintenanceRequest) (*entity.Maintenance, error) {
    ctx = xlog.WithOperation(ctx, "service.Maintenance.Create")

    // Validate input
    if err := s.validateCreateRequest(req); err != nil {
        xlog.Warn(ctx, "validation failed", zap.Error(err))
        return nil, err  // Validation errors are expected, don't wrap
    }

    // Check for conflicts
    conflicts, err := s.repo.FindConflicts(ctx, req.PlannedPeriod, req.ResourceIDs)
    if err != nil {
        xlog.Error(ctx, "failed to check conflicts", zap.Error(err))
        return nil, fmt.Errorf("check conflicts: %w", err)
    }

    if len(conflicts) > 0 {
        xlog.Warn(ctx, "maintenance conflicts detected", zap.Int("count", len(conflicts)))
        return nil, apperr.ErrConflict
    }

    // Create entity
    maint := &entity.Maintenance{
        ID:            uuid.New(),
        Title:         req.Title,
        Description:   req.Description,
        PlannedPeriod: req.PlannedPeriod,
        Status:        entity.MaintenanceStatusDraft,
    }

    // Save to repository
    if err := s.repo.Create(ctx, maint); err != nil {
        xlog.Error(ctx, "failed to create maintenance",
            zap.Error(err),
            zap.String("title", req.Title),
        )
        return nil, fmt.Errorf("create maintenance: %w", err)
    }

    xlog.Info(ctx, "maintenance created successfully", zap.String("id", maint.ID.String()))
    return maint, nil
}

func (s *Service) UpdateMaintenanceStatus(ctx context.Context, id uuid.UUID, newStatus entity.MaintenanceStatus) error {
    ctx = xlog.WithOperation(ctx, "service.Maintenance.UpdateStatus")

    // Get current maintenance
    maint, err := s.repo.Get(ctx, id)
    if err != nil {
        if errors.Is(err, apperr.ErrMaintNotFound) {
            xlog.Warn(ctx, "maintenance not found", zap.String("id", id.String()))
            return err
        }
        xlog.Error(ctx, "failed to get maintenance", zap.Error(err))
        return fmt.Errorf("get maintenance: %w", err)
    }

    // Validate status transition
    if !isValidTransition(maint.Status, newStatus) {
        xlog.Warn(ctx, "invalid status transition",
            zap.String("from", string(maint.Status)),
            zap.String("to", string(newStatus)),
        )
        return apperr.ForbiddenStatusTransition(maint.Status)
    }

    // Update status
    maint.Status = newStatus
    if err := s.repo.Update(ctx, maint); err != nil {
        xlog.Error(ctx, "failed to update maintenance status",
            zap.Error(err),
            zap.String("id", id.String()),
            zap.String("new_status", string(newStatus)),
        )
        return fmt.Errorf("update maintenance status: %w", err)
    }

    xlog.Info(ctx, "maintenance status updated",
        zap.String("id", id.String()),
        zap.String("new_status", string(newStatus)),
    )
    return nil
}

func (s *Service) validateCreateRequest(req *CreateMaintenanceRequest) error {
    if req.Title == "" {
        return fmt.Errorf("%w: title is required", apperr.ErrValidation)
    }

    if req.PlannedPeriod.Start.IsZero() || req.PlannedPeriod.End.IsZero() {
        return apperr.ErrInvalidPeriodEmptyStartOrEnd
    }

    if req.PlannedPeriod.Start.After(req.PlannedPeriod.End) ||
        req.PlannedPeriod.Start.Equal(req.PlannedPeriod.End) {
        return apperr.ErrInvalidPeriodStartOrEnd
    }

    return nil
}

func isValidTransition(from, to entity.MaintenanceStatus) bool {
    validTransitions := map[entity.MaintenanceStatus][]entity.MaintenanceStatus{
        entity.MaintenanceStatusDraft: {
            entity.MaintenanceStatusPlanned,
            entity.MaintenanceStatusCancelled,
        },
        entity.MaintenanceStatusPlanned: {
            entity.MaintenanceStatusInProgress,
            entity.MaintenanceStatusCancelled,
        },
        entity.MaintenanceStatusInProgress: {
            entity.MaintenanceStatusCompleted,
            entity.MaintenanceStatusCancelled,
        },
    }

    allowed, exists := validTransitions[from]
    if !exists {
        return false
    }

    for _, status := range allowed {
        if status == to {
            return true
        }
    }
    return false
}
```

## Example 2: Repository with Error Handling

```go
package maintenance

import (
    "context"
    "errors"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/ruko1202/maintmode/internal/apperr"
    "github.com/ruko1202/maintmode/internal/entity"
)

type Repository struct {
    db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
    return &Repository{db: db}
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*entity.Maintenance, error) {
    const query = `
        SELECT id, title, description, planned_start, planned_end,
               actual_start, actual_end, status, created_at, updated_at
        FROM maintenances
        WHERE id = $1
    `

    var m entity.Maintenance
    err := r.db.QueryRow(ctx, query, id).Scan(
        &m.ID, &m.Title, &m.Description,
        &m.PlannedPeriod.Start, &m.PlannedPeriod.End,
        &m.ActualPeriod.Start, &m.ActualPeriod.End,
        &m.Status, &m.CreatedAt, &m.UpdatedAt,
    )

    if err != nil {
        // Convert database-specific errors to domain errors
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, apperr.ErrMaintNotFound
        }
        return nil, fmt.Errorf("query maintenance: %w", err)
    }

    return &m, nil
}

func (r *Repository) Create(ctx context.Context, m *entity.Maintenance) error {
    const query = `
        INSERT INTO maintenances (id, title, description, planned_start, planned_end, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
    `

    _, err := r.db.Exec(ctx, query,
        m.ID, m.Title, m.Description,
        m.PlannedPeriod.Start, m.PlannedPeriod.End,
        m.Status,
    )

    if err != nil {
        return fmt.Errorf("insert maintenance: %w", err)
    }

    return nil
}

func (r *Repository) Update(ctx context.Context, m *entity.Maintenance) error {
    const query = `
        UPDATE maintenances
        SET title = $2, description = $3, planned_start = $4, planned_end = $5,
            actual_start = $6, actual_end = $7, status = $8, updated_at = NOW()
        WHERE id = $1
    `

    result, err := r.db.Exec(ctx, query,
        m.ID, m.Title, m.Description,
        m.PlannedPeriod.Start, m.PlannedPeriod.End,
        m.ActualPeriod.Start, m.ActualPeriod.End,
        m.Status,
    )

    if err != nil {
        return fmt.Errorf("update maintenance: %w", err)
    }

    if result.RowsAffected() == 0 {
        return apperr.ErrMaintNotFound
    }

    return nil
}

func (r *Repository) FindConflicts(ctx context.Context, period entity.Period, resourceIDs []uuid.UUID) ([]*entity.Maintenance, error) {
    const query = `
        SELECT DISTINCT m.id, m.title, m.planned_start, m.planned_end, m.status
        FROM maintenances m
        JOIN maintenance_resources mr ON m.id = mr.maintenance_id
        WHERE mr.resource_id = ANY($1)
          AND m.status IN ('planned', 'in_progress')
          AND (m.planned_start, m.planned_end) OVERLAPS ($2, $3)
    `

    rows, err := r.db.Query(ctx, query, resourceIDs, period.Start, period.End)
    if err != nil {
        return nil, fmt.Errorf("query conflicts: %w", err)
    }
    defer rows.Close()

    var conflicts []*entity.Maintenance
    for rows.Next() {
        var m entity.Maintenance
        if err := rows.Scan(&m.ID, &m.Title, &m.PlannedPeriod.Start, &m.PlannedPeriod.End, &m.Status); err != nil {
            return nil, fmt.Errorf("scan conflict: %w", err)
        }
        conflicts = append(conflicts, &m)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("iterate conflicts: %w", err)
    }

    return conflicts, nil
}
```

## Example 3: Echo Handler with Comprehensive Error Handling

```go
package uicalendar

import (
    "errors"
    "net/http"

    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "github.com/ruko1202/xlog"
    "go.uber.org/zap"

    "github.com/ruko1202/maintmode/internal/apperr"
    "github.com/ruko1202/maintmode/internal/app/api/apierrors"
    "github.com/ruko1202/maintmode/internal/entity"
)

type Handler struct {
    service *maintenance.Service
}

func NewHandler(service *maintenance.Service) *Handler {
    return &Handler{service: service}
}

// GetMaintenance godoc
// @Summary Get maintenance by ID
// @Tags Maintenance
// @Param id path string true "Maintenance ID" Format(uuid)
// @Success 200 {object} MaintenanceResponse
// @Failure 400 {object} apierrors.ErrorResponse
// @Failure 404 {object} apierrors.ErrorResponse
// @Failure 500 {object} apierrors.ErrorResponse
// @Router /maintenances/{id} [get]
func (h *Handler) GetMaintenance(c echo.Context) error {
    ctx := xlog.WithOperation(c.Request().Context(), "api.GetMaintenance")

    // Parse and validate ID
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        xlog.Warn(ctx, "invalid UUID format", zap.Error(err), zap.String("id", c.Param("id")))
        return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            "Maintenance ID must be a valid UUID",
        ))
    }

    // Get maintenance from service
    maint, err := h.service.GetMaintenance(ctx, id)
    if err != nil {
        return h.handleServiceError(c, err, "get maintenance")
    }

    return c.JSON(http.StatusOK, toMaintenanceResponse(maint))
}

// CreateMaintenance godoc
// @Summary Create new maintenance
// @Tags Maintenance
// @Accept json
// @Produce json
// @Param request body CreateMaintenanceRequest true "Maintenance data"
// @Success 201 {object} MaintenanceResponse
// @Failure 400 {object} apierrors.ErrorResponse
// @Failure 422 {object} apierrors.ErrorResponse
// @Failure 500 {object} apierrors.ErrorResponse
// @Router /maintenances [post]
func (h *Handler) CreateMaintenance(c echo.Context) error {
    ctx := xlog.WithOperation(c.Request().Context(), "api.CreateMaintenance")

    // Bind and validate request
    var req CreateMaintenanceRequest
    if err := c.Bind(&req); err != nil {
        xlog.Warn(ctx, "bind request failed", zap.Error(err))
        return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            "Invalid request body",
        ))
    }

    // Create maintenance
    maint, err := h.service.CreateMaintenance(ctx, toCreateMaintenanceServiceRequest(&req))
    if err != nil {
        return h.handleServiceError(c, err, "create maintenance")
    }

    return c.JSON(http.StatusCreated, toMaintenanceResponse(maint))
}

// UpdateMaintenanceStatus godoc
// @Summary Update maintenance status
// @Tags Maintenance
// @Param id path string true "Maintenance ID" Format(uuid)
// @Param request body UpdateStatusRequest true "New status"
// @Success 200 {object} MaintenanceResponse
// @Failure 400 {object} apierrors.ErrorResponse
// @Failure 403 {object} apierrors.ErrorResponse
// @Failure 404 {object} apierrors.ErrorResponse
// @Failure 500 {object} apierrors.ErrorResponse
// @Router /maintenances/{id}/status [patch]
func (h *Handler) UpdateMaintenanceStatus(c echo.Context) error {
    ctx := xlog.WithOperation(c.Request().Context(), "api.UpdateMaintenanceStatus")

    // Parse ID
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        xlog.Warn(ctx, "invalid UUID format", zap.Error(err))
        return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            "Maintenance ID must be a valid UUID",
        ))
    }

    // Bind request
    var req UpdateStatusRequest
    if err := c.Bind(&req); err != nil {
        xlog.Warn(ctx, "bind request failed", zap.Error(err))
        return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            "Invalid request body",
        ))
    }

    // Update status
    if err := h.service.UpdateMaintenanceStatus(ctx, id, req.Status); err != nil {
        return h.handleServiceError(c, err, "update maintenance status")
    }

    // Get updated maintenance
    maint, err := h.service.GetMaintenance(ctx, id)
    if err != nil {
        return h.handleServiceError(c, err, "get updated maintenance")
    }

    return c.JSON(http.StatusOK, toMaintenanceResponse(maint))
}

// handleServiceError converts service errors to HTTP responses
func (h *Handler) handleServiceError(c echo.Context, err error, operation string) error {
    ctx := c.Request().Context()

    switch {
    case errors.Is(err, apperr.ErrMaintNotFound):
        xlog.Warn(ctx, operation+" not found", zap.Error(err))
        return c.JSON(http.StatusNotFound, apierrors.NewErrorResponse(
            apierrors.ErrNotFound,
            "Maintenance window not found",
        ))

    case errors.Is(err, apperr.ErrResourceNotFound):
        xlog.Warn(ctx, operation+" resource not found", zap.Error(err))
        return c.JSON(http.StatusNotFound, apierrors.NewErrorResponse(
            apierrors.ErrNotFound,
            "Resource not found",
        ))

    case errors.Is(err, apperr.ErrInvalidPeriodEmptyStartOrEnd):
        xlog.Warn(ctx, operation+" validation failed", zap.Error(err))
        return c.JSON(http.StatusUnprocessableEntity, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            "Start and end times are required",
        ))

    case errors.Is(err, apperr.ErrInvalidPeriodStartOrEnd):
        xlog.Warn(ctx, operation+" validation failed", zap.Error(err))
        return c.JSON(http.StatusUnprocessableEntity, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            "Start time must be before end time",
        ))

    case errors.Is(err, apperr.ErrForbiddenStatusTransition):
        xlog.Warn(ctx, operation+" forbidden", zap.Error(err))
        return c.JSON(http.StatusForbidden, apierrors.NewErrorResponse(
            apierrors.ErrForbiddenStatusTransition,
            err.Error(),
        ))

    case errors.Is(err, apperr.ErrConflict):
        xlog.Warn(ctx, operation+" conflict", zap.Error(err))
        return c.JSON(http.StatusConflict, apierrors.NewErrorResponse(
            apierrors.ErrConflict,
            "Maintenance window conflicts with existing windows",
        ))

    default:
        xlog.Error(ctx, operation+" failed", zap.Error(err))
        return c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
            apierrors.ErrInternalError,
            "Failed to "+operation,
        ))
    }
}
```

## Example 4: Transaction Error Handling

```go
package maintenance

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/ruko1202/xlog"
    "go.uber.org/zap"
)

func (s *Service) CreateMaintenanceWithResources(
    ctx context.Context,
    req *CreateMaintenanceRequest,
) (*entity.Maintenance, error) {
    ctx = xlog.WithOperation(ctx, "service.Maintenance.CreateWithResources")

    // Begin transaction
    tx, err := s.repo.BeginTx(ctx)
    if err != nil {
        xlog.Error(ctx, "failed to begin transaction", zap.Error(err))
        return nil, fmt.Errorf("begin transaction: %w", err)
    }
    defer func() {
        if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
            xlog.Error(ctx, "failed to rollback transaction", zap.Error(err))
        }
    }()

    // Create maintenance
    maint := &entity.Maintenance{
        ID:            uuid.New(),
        Title:         req.Title,
        Description:   req.Description,
        PlannedPeriod: req.PlannedPeriod,
        Status:        entity.MaintenanceStatusDraft,
    }

    if err := s.repo.CreateTx(ctx, tx, maint); err != nil {
        xlog.Error(ctx, "failed to create maintenance in tx", zap.Error(err))
        return nil, fmt.Errorf("create maintenance: %w", err)
    }

    // Add resources
    for _, resourceID := range req.ResourceIDs {
        if err := s.repo.AddResourceTx(ctx, tx, maint.ID, resourceID); err != nil {
            xlog.Error(ctx, "failed to add resource in tx",
                zap.Error(err),
                zap.String("resource_id", resourceID.String()),
            )
            return nil, fmt.Errorf("add resource: %w", err)
        }
    }

    // Commit transaction
    if err := tx.Commit(ctx); err != nil {
        xlog.Error(ctx, "failed to commit transaction", zap.Error(err))
        return nil, fmt.Errorf("commit transaction: %w", err)
    }

    xlog.Info(ctx, "maintenance with resources created successfully",
        zap.String("id", maint.ID.String()),
        zap.Int("resources", len(req.ResourceIDs)),
    )

    return maint, nil
}
```

## Example 5: Testing Error Scenarios

```go
package maintenance_test

import (
    "context"
    "errors"
    "testing"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"

    "github.com/ruko1202/maintmode/internal/apperr"
    "github.com/ruko1202/maintmode/internal/entity"
)

// Mock repository
type MockRepository struct {
    mock.Mock
}

func (m *MockRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Maintenance, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*entity.Maintenance), args.Error(1)
}

// Test: Not found error
func TestService_GetMaintenance_NotFound(t *testing.T) {
    repo := new(MockRepository)
    repo.On("Get", mock.Anything, mock.Anything).
        Return(nil, apperr.ErrMaintNotFound)

    service := NewService(repo, zap.NewNop())

    _, err := service.GetMaintenance(context.Background(), uuid.New())

    require.Error(t, err)
    assert.True(t, errors.Is(err, apperr.ErrMaintNotFound))
    repo.AssertExpectations(t)
}

// Test: Unexpected error wrapping
func TestService_GetMaintenance_UnexpectedError(t *testing.T) {
    dbErr := errors.New("database connection failed")
    repo := new(MockRepository)
    repo.On("Get", mock.Anything, mock.Anything).
        Return(nil, dbErr)

    service := NewService(repo, zap.NewNop())

    _, err := service.GetMaintenance(context.Background(), uuid.New())

    require.Error(t, err)
    assert.True(t, errors.Is(err, dbErr))
    assert.Contains(t, err.Error(), "get maintenance")
    repo.AssertExpectations(t)
}

// Test: Validation error
func TestService_CreateMaintenance_ValidationError(t *testing.T) {
    repo := new(MockRepository)
    service := NewService(repo, zap.NewNop())

    req := &CreateMaintenanceRequest{
        Title: "", // Invalid: empty title
    }

    _, err := service.CreateMaintenance(context.Background(), req)

    require.Error(t, err)
    assert.True(t, errors.Is(err, apperr.ErrValidation))
}

// Test: Status transition error
func TestService_UpdateStatus_ForbiddenTransition(t *testing.T) {
    maint := &entity.Maintenance{
        ID:     uuid.New(),
        Status: entity.MaintenanceStatusCompleted,
    }

    repo := new(MockRepository)
    repo.On("Get", mock.Anything, maint.ID).Return(maint, nil)

    service := NewService(repo, zap.NewNop())

    err := service.UpdateMaintenanceStatus(
        context.Background(),
        maint.ID,
        entity.MaintenanceStatusDraft, // Invalid transition
    )

    require.Error(t, err)
    assert.True(t, errors.Is(err, apperr.ErrForbiddenStatusTransition))
    repo.AssertExpectations(t)
}
```

## Example 6: Error Handling in Middleware

```go
package middlewares

import (
    "errors"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/ruko1202/xlog"
    "go.uber.org/zap"

    "github.com/ruko1202/maintmode/internal/apperr"
    "github.com/ruko1202/maintmode/internal/app/api/apierrors"
)

func ErrorLoggingMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            start := time.Now()

            err := next(c)

            duration := time.Since(start)
            ctx := c.Request().Context()

            if err != nil {
                status := c.Response().Status

                // Prepare log fields
                fields := []zap.Field{
                    zap.String("method", c.Request().Method),
                    zap.String("path", c.Request().URL.Path),
                    zap.Int("status", status),
                    zap.Duration("duration", duration),
                    zap.Error(err),
                }

                // Log based on error type
                switch {
                case errors.Is(err, apperr.ErrNotFound):
                    xlog.Warn(ctx, "resource not found", fields...)
                case errors.Is(err, apperr.ErrValidation):
                    xlog.Warn(ctx, "validation error", fields...)
                case errors.Is(err, apperr.ErrForbidden):
                    xlog.Warn(ctx, "forbidden access", fields...)
                case status >= 500:
                    xlog.Error(ctx, "server error", fields...)
                default:
                    xlog.Info(ctx, "request completed", fields...)
                }
            }

            return err
        }
    }
}
```
