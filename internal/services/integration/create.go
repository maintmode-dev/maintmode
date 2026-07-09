package integration

import (
	"context"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/audit"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Create validates the config/secrets for the kind, encrypts the secret values
// under a fresh DEK, and stores the integration — all in one transaction. Config
// is stored as the raw JSON the caller supplied; secrets belong in the separate,
// encrypted Secrets field. Returns the masked view (never plaintext secrets).
func (s *Service) Create(ctx context.Context, cmd *entity.CreateIntegrationCmd) (*entity.MaskedIntegration, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.Create",
		xfield.String("kind", cmd.Kind),
	)
	defer span.End()

	if err := validateCreateIntegrationCmd(cmd); err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	// Decode the caller's secrets object at the service boundary: the API layer
	// passes the raw JSON through untyped.
	plain, err := cmd.PlainSecrets()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	in, err := s.validateKind(cmd.Kind, cmd.Config, plain)
	if err != nil {
		return nil, err
	}
	// Undeclared keys are dropped, not stored; the warning is the only trace a
	// typoed key name leaves.
	warnUnknownSecretKeys(ctx, in, plain)

	var created *entity.IntegrationSetting
	err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		dek, dekID, dekErr := s.newDEK(ctx)
		if dekErr != nil {
			return dekErr
		}
		encrypted, encErr := s.encryptSecrets(in, dek, plain)
		if encErr != nil {
			return encErr
		}

		created, err = s.store.Create(ctx, &entity.IntegrationSetting{
			Kind:            cmd.Kind,
			Enabled:         lo.FromPtr(cmd.Enabled),
			Config:          cmd.Config,
			Secrets:         encrypted,
			DEKID:           dekID,
			CreatedByUserID: &cmd.Actor.ID,
		})

		s.publishAudit(ctx, audit.IntegrationCreated{Actor: cmd.Actor, Kind: cmd.Kind, Enabled: *cmd.Enabled})
		return err
	})
	if err != nil {
		xlog.Error(ctx, "create integration failed",
			xfield.String("kind", cmd.Kind), xfield.Error(err))
		return nil, err
	}

	// Tell the delivery side the stored state changed (post-commit; see
	// notifyChanged for why not inside the tx).
	s.notifyChanged(cmd.Kind)

	return created.Mask(), nil
}

func validateCreateIntegrationCmd(cmd *entity.CreateIntegrationCmd) error {
	// Config and Secrets are deliberately NOT required: whether either is needed
	// is the kind's decision (Parse/Validate) — resolvable/slack work with no
	// config, an unauthenticated SMTP relay with no secrets.
	//
	// NotNil, not Required, for the pointers: ozzo's Required dereferences a
	// pointer and treats the zero value as empty, which would reject
	// Enabled=&false — making it impossible to create a disabled integration.
	return validation.ValidateStruct(cmd,
		validation.Field(&cmd.Kind, validation.Required),
		validation.Field(&cmd.Enabled, validation.NotNil.Error("enabled must be set explicitly")),
		validation.Field(&cmd.Actor, validation.NotNil.Error("actor is required")),
	)
}
