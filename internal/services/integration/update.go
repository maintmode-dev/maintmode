package integration

import (
	"context"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Update patches an integration: every omitted field keeps its stored value —
// PATCH semantics uniformly, matching the route. Enabled: nil keeps the current
// flag. Config: nil keeps the stored config; an explicit object (including {})
// replaces it wholesale. Secrets, per key: omitted keeps the stored ciphertext
// (no decrypt), a non-empty value is re-encrypted, null/"" clears it (e.g.
// dropping SMTP auth). The DEK is reused — kept coupled to the ciphertext it
// protects, never repointed. All in one transaction; returns the masked view.
func (s *Service) Update(ctx context.Context, cmd *entity.UpdateIntegrationCmd) (*entity.MaskedIntegration, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.Update",
		xfield.String("kind", cmd.Kind),
	)
	defer span.End()

	if err := validateUpdateIntegrationCmd(cmd); err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	// Decode the caller's secrets object at the service boundary, preserving the
	// per-key intent (absent/replace/null=clear) that the raw JSON carries.
	intents, err := cmd.SecretIntents()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	in, err := s.registry.Get(cmd.Kind)
	if err != nil {
		return nil, err
	}
	// Undeclared keys are dropped by the merge below, not stored; the warning is
	// the only trace a typoed key name leaves.
	warnUnknownSecretKeys(ctx, in, intents)

	integration, err := s.updateWithApply(ctx, cmd.Kind, func(ctx context.Context, current *entity.IntegrationSetting) error {
		merged, mergeErr := s.mergeSecrets(ctx, in, current, intents)
		if mergeErr != nil {
			return mergeErr
		}

		applyUpdateIntegrationCmd(current, cmd)
		current.Secrets = merged.stored
		current.UpdatedByUserID = &cmd.Actor.ID

		// Re-validate the EFFECTIVE state (post-patch config + merged secrets) so
		// an update can't leave the integration invalid (e.g. a config edit that
		// drops a field a still-present secret needs). Carried-over secrets are
		// validated by presence, not value, so an unchanged secret need not be
		// decrypted.
		if _, valErr := s.validateKind(cmd.Kind, current.Config, merged.view); valErr != nil {
			return valErr
		}

		s.publishAudit(ctx, audit.IntegrationUpdated{Actor: cmd.Actor, Kind: cmd.Kind, Enabled: current.Enabled})
		return nil
	})

	if err != nil {
		xlog.Error(ctx, "update integration failed",
			xfield.String("kind", cmd.Kind), xfield.Error(err))
		return nil, err
	}

	return integration.Mask(), nil
}

func (s *Service) updateWithApply(ctx context.Context, kind string,
	fn func(ctx context.Context, current *entity.IntegrationSetting) error,
) (*entity.IntegrationSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.applyIntegrationForUpdate")
	defer span.End()

	var integration *entity.IntegrationSetting
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		current, err := s.store.GetForUpdateByKind(ctx, kind)
		if err != nil {
			return fmt.Errorf("get integration: %w", err)
		}

		if err = fn(ctx, current); err != nil {
			return fmt.Errorf("apply integration update: %w", err)
		}

		integration, err = s.store.Update(ctx, current)
		if err != nil {
			return fmt.Errorf("update integration: %w", err)
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to update integration", xfield.Error(err))
		return nil, err
	}

	s.notifyChanged(kind)

	return integration, nil
}

// applyUpdateIntegrationCmd PATCH semantics: only fields the caller sent change. A nil Config is
// "unchanged" — distinct from an explicit {} which replaces it — so a
// stored value can never be wiped by an omission.
func applyUpdateIntegrationCmd(current *entity.IntegrationSetting, cmd *entity.UpdateIntegrationCmd) {
	if cmd.Enabled != nil {
		current.Enabled = *cmd.Enabled
	}
	if cmd.Config != nil {
		current.Config = cmd.Config
	}
}

func validateUpdateIntegrationCmd(cmd *entity.UpdateIntegrationCmd) error {
	// Unlike create, Enabled is NOT required: PATCH semantics — an omitted field
	// keeps the stored value, and here there IS a stored value to keep (at create
	// there is no prior state, so the choice must be explicit). Config and
	// Secrets are likewise optional; the effective post-patch state is
	// re-validated by the kind.
	return validation.ValidateStruct(cmd,
		validation.Field(&cmd.Kind, validation.Required),
		validation.Field(&cmd.Actor, validation.NotNil.Error("actor is required")),
	)
}
