package integration

import (
	"context"
	"fmt"
	"strings"

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
	// Trimmed before anything reads it, so the stored name, the span, the audit
	// entry and the URL that will address this row all say the same thing. The
	// alternative -- refusing a name with surrounding space -- makes the caller
	// fix something no one meant to type, and " google " and "google" are not
	// two names an operator would ever want to tell apart.
	cmd.Name = strings.TrimSpace(cmd.Name)

	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.Create",
		xfield.String("kind", cmd.Kind),
		xfield.String("name", cmd.Name),
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

	// The pair is the whole name rule. Every shape a free-text check would have
	// refused -- blank, a slash, an overlong string, a reserved word like
	// "bootstrap" -- is not in the registry, so this refuses it by the same
	// lookup that decides which implementation parses the row. An operator does
	// not name an integration; they pick one of a closed set of five.
	if err := s.registry.admit(cmd.Kind, cmd.Name); err != nil {
		return nil, err
	}

	// The catalog's values are written into the config BEFORE validation and
	// before the secret is sealed, so the stored row is self-contained and the
	// AAD is computed from what is about to be saved. Doing it later would seal
	// a secret against an issuer the row does not yet carry.
	cfg, err := s.applyPreset(cmd.Name, cmd.Config)
	if err != nil {
		return nil, err
	}
	cmd.Config = cfg

	in, settings, err := s.resolveSettings(cmd.Name, cmd.Config, plain)
	if err != nil {
		return nil, err
	}
	// Undeclared keys are dropped, not stored; the warning is the only trace a
	// typoed key name leaves.
	warnUnknownSecretKeys(ctx, in, plain)

	var created *entity.IntegrationSetting
	err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		// No lock and no linked-account check here, and both used to be: the
		// check refused a name that identities already carried, and the lock
		// kept its count from being separated from the insert.
		//
		// What made them necessary was a name that could mean a different IdP
		// later. The CLOSED SET removed that: the only login names are `google`,
		// whose issuer the preset fixes so it cannot be aimed anywhere, and
		// `custom`, which an admin may re-point at will -- refusing to create it
		// bought nothing they could not do by editing the row instead. Two
		// concurrent creates of one name are settled by UNIQUE (kind, name),
		// which the store already reports as ErrIntegrationConflict.
		dek, dekID, dekErr := s.newDEK(ctx)
		if dekErr != nil {
			return dekErr
		}
		encrypted, encErr := s.encryptSecrets(in, settings, dek, plain)
		if encErr != nil {
			return encErr
		}

		created, err = s.store.Create(ctx, &entity.IntegrationSetting{
			Kind:            cmd.Kind,
			Name:            cmd.Name,
			Enabled:         lo.FromPtr(cmd.Enabled),
			Config:          cmd.Config,
			Secrets:         encrypted,
			DEKID:           dekID,
			CreatedByUserID: &cmd.Actor.ID,
		})

		s.publishAudit(ctx, audit.IntegrationCreated{Actor: cmd.Actor, Kind: cmd.Kind, Name: cmd.Name, Enabled: *cmd.Enabled})
		return err
	})
	if err != nil {
		xlog.Error(ctx, "create integration failed",
			xfield.String("kind", cmd.Kind), xfield.Error(err))
		return nil, err
	}

	// Tell the delivery side the stored state changed (post-commit; see
	// notifyChanged for why not inside the tx).
	s.notifyChanged(cmd.Kind, cmd.Name)

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
