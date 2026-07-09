package integration

import (
	"context"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/samber/lo"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Toggle flips the enabled flag of an integration at runtime, leaving config and
// secrets untouched. It is the fast path for "turn this integration on/off"
// without resending settings. Runs in a transaction; returns the masked view.
func (s *Service) Toggle(ctx context.Context, cmd *entity.ToggleIntegrationCmd) (*entity.MaskedIntegration, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.Toggle",
		xfield.String("kind", cmd.Kind),
	)
	defer span.End()

	if err := validateToggleIntegrationCmd(cmd); err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	integration, err := s.updateWithApply(ctx, cmd.Kind, func(_ context.Context, current *entity.IntegrationSetting) error {
		current.Enabled = lo.FromPtr(cmd.Enabled)
		current.UpdatedByUserID = &cmd.Actor.ID

		s.publishAudit(ctx, audit.IntegrationUpdated{Actor: cmd.Actor, Kind: cmd.Kind, Enabled: lo.FromPtr(cmd.Enabled)})
		return nil
	})

	if err != nil {
		xlog.Error(ctx, "toggle integration failed", xfield.Error(err))
		return nil, err
	}

	return integration.Mask(), nil
}

func validateToggleIntegrationCmd(cmd *entity.ToggleIntegrationCmd) error {
	// Enabled is a plain bool here: toggling carries the target state by value,
	// there is no "omitted" to distinguish. NotNil on Actor — see
	// validateCreateIntegrationCmd for why not Required.
	return validation.ValidateStruct(cmd,
		validation.Field(&cmd.Kind, validation.Required),
		validation.Field(&cmd.Enabled, validation.NotNil.Error("enabled must be set explicitly")),
		validation.Field(&cmd.Actor, validation.NotNil.Error("actor is required")),
	)
}
