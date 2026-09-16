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
		xfield.String("name", cmd.Name),
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

	in, err := s.registry.get(cmd.Name)
	if err != nil {
		return nil, err
	}
	// Undeclared keys are dropped by the merge below, not stored; the warning is
	// the only trace a typoed key name leaves.
	warnUnknownSecretKeys(ctx, in, intents)

	integration, err := s.updateWithApply(ctx, cmd.Kind, cmd.Name, func(ctx context.Context, current *entity.IntegrationSetting) error {
		// Reads the STORED config, before the patch is applied: this is the only
		// point where the old and new states both exist.
		//
		// Re-pointing a provider people already sign in through is NOT refused.
		// It was, and the refusal was wrong: an IdP migration is something an
		// admin is entitled to do, and there was no way to say "I know" -- the
		// only path past it was to unlink every account by hand, which is worse
		// than the thing being prevented. Changing `custom` is an admin
		// operation, and admin means admin. What still cannot move is a preset
		// provider's issuer: applyPreset and enforcePreset own those fields, so
		// `google` cannot be aimed elsewhere whatever the caller sends.
		//
		// Checked FIRST of the two, and the order is about which reason an
		// operator is told. Both look at the same incoming config, but this one
		// asks whether the change is allowed at all while the AAD check asks
		// whether the stored ciphertext would survive it -- cause before
		// consequence. Ordered the other way round, aiming `google` at another
		// issuer without resending the secret answered "supply client_secret in
		// the same request": true, useless, and pointing at the one edit that
		// would NOT help.
		//
		// A config the caller did not send cannot have moved a preset field,
		// and there is nothing to compare against in that case.
		if cmd.Config != nil {
			if presetErr := s.enforcePreset(cmd.Name, cmd.Config); presetErr != nil {
				return presetErr
			}
		}

		// The check below is a different question: not "may this change happen"
		// but "will the stored ciphertext survive it". It stays for the rows the
		// rule above does not speak for -- `custom`, where re-pointing IS an
		// admin's call and the only question left is the secret.
		if aadErr := s.checkAADBindingStable(in, current, cmd, intents); aadErr != nil {
			return aadErr
		}

		// The patch lands BEFORE the merge, and the order is load-bearing for
		// login providers: their secrets are sealed under identifiers taken from
		// the config, so sealing against the pre-patch config would store a
		// ciphertext bound to values the row no longer holds — unopenable from
		// the moment it is written. The merge must see the effective state.
		//
		// Nothing else depends on the old order: the merge takes its DEK from
		// current.DEKID, which the patch does not touch.
		applyUpdateIntegrationCmd(current, cmd)

		merged, mergeErr := s.mergeSecrets(ctx, in, current, intents)
		if mergeErr != nil {
			return mergeErr
		}

		current.Secrets = merged.stored
		current.UpdatedByUserID = &cmd.Actor.ID

		// Re-validate the EFFECTIVE state (post-patch config + merged secrets) so
		// an update can't leave the integration invalid (e.g. a config edit that
		// drops a field a still-present secret needs). Carried-over secrets are
		// validated by presence, not value, so an unchanged secret need not be
		// decrypted.
		if _, _, valErr := s.resolveSettings(cmd.Name, current.Config, merged.view); valErr != nil {
			return valErr
		}

		s.publishAudit(ctx, audit.IntegrationUpdated{Actor: cmd.Actor, Kind: cmd.Kind, Name: cmd.Name, Enabled: current.Enabled})
		return nil
	})

	if err != nil {
		xlog.Error(ctx, "update integration failed",
			xfield.String("kind", cmd.Kind), xfield.Error(err))
		return nil, err
	}

	return integration.Mask(), nil
}

func (s *Service) updateWithApply(ctx context.Context, kind, name string,
	fn func(ctx context.Context, current *entity.IntegrationSetting) error,
) (*entity.IntegrationSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.applyIntegrationForUpdate",
		xfield.String("kind", kind),
		xfield.String("name", name),
	)
	defer span.End()

	var integration *entity.IntegrationSetting
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		current, err := s.store.GetForUpdateByKindName(ctx, kind, name)
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

	s.notifyChanged(kind, name)

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
