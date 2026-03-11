package maint

import (
	"context"
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) CreateDraft(ctx context.Context, cmd *entity.CreateMaintenanceCmd) (*entity.Maintenance, error) {
	ctx = xlog.WithOperation(ctx, "service.Maint.CreateDraft")

	if err := validateCreate(cmd); err != nil {
		return nil, err
	}

	maint := &entity.Maintenance{
		ID:            xuuid.New(),
		Title:         cmd.Title,
		Description:   cmd.Description,
		PlannedPeriod: cmd.PlannedPeriod,
		Resources:     cmd.Resources,
		Scope:         cmd.Scope,
		Impact:        cmd.Impact,
		Status:        entity.MaintenanceStatusDraft,
		CreatedAt:     xtime.UTCNow(),
	}

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		err := s.maintStore.Create(ctx, maint)
		if err != nil {
			xlog.Error(ctx, "create maint failed", xfield.Error(err))
			return err
		}
		if len(maint.Resources) > 0 {
			err = s.maintStore.AddResources(ctx, maint.ID, maint.Resources)
			if err != nil {
				xlog.Error(ctx, "attach resources to maint failed", xfield.Error(err))
				return err
			}
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "create maint failed", xfield.Error(err))
		return nil, err
	}

	return maint, nil
}

func validateCreate(cmd *entity.CreateMaintenanceCmd) error {
	return validation.ValidateStruct(cmd,
		validation.Field(&cmd.Title, validation.Required),
		validation.Field(&cmd.Resources, validation.Required.When(cmd.Scope == entity.MaintenanceScopeResources)),
		validation.Field(&cmd.Description, validation.Required),
		validation.Field(&cmd.PlannedPeriod, validation.Required, validation.WithContext(validatePlanedPeriod)),
		validation.Field(&cmd.Impact, validation.Required),
	)
}

func validatePlanedPeriod(_ context.Context, value any) error {
	var start, end time.Time

	switch v := value.(type) {
	case *entity.Period:
		start, end = v.Start, lo.FromPtr(v.End)
	case entity.Period:
		start, end = v.Start, lo.FromPtr(v.End)
	default:
		return fmt.Errorf("invalid period type: %T", v)
	}

	if start.IsZero() || end.IsZero() {
		return apperr.ErrInvalidPeriodEmptyStartOrEnd
	}
	if start.After(end) || start.Equal(end) {
		return apperr.ErrInvalidPeriodStartOrEnd
	}

	return nil
}
