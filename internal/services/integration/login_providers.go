package integration

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// ListLoginProviders reads every stored row of a kind, decrypting what it can.
//
// It is a separate read path rather than a loop over Settings for two reasons
// that both matter to the reloader. Settings refuses a disabled row with
// ErrIntegrationDisabled, and the reloader needs those to know what to stop
// serving; and Settings fails the whole call on the first unreadable row, where
// the reloader must keep the other providers working -- one broken provider
// disabling itself is the point, one broken provider disabling login is the
// outage.
func (s *Service) ListLoginProviders(ctx context.Context, kind string) ([]entity.ConfiguredProvider, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.ListLoginProviders",
		xfield.String("kind", kind),
	)
	defer span.End()

	rows, err := s.store.ListByKind(ctx, kind)
	if err != nil {
		return nil, fmt.Errorf("list %q: %w", kind, err)
	}

	providers := make([]entity.ConfiguredProvider, 0, len(rows))
	for _, row := range rows {
		providers = append(providers, s.openProvider(ctx, row))
	}

	return providers, nil
}

// openProvider turns one stored row into what a reload needs from it.
//
// Every way of failing lands in ONE place: the row is reported with Unreadable
// set, never dropped and never fatal to the call. Each failure was its own
// early return before, which meant the flag and the append were written twice
// and a third failure mode would have had to remember both.
//
// The error is logged here, where it happened, and goes no further: the caller
// gets a fact about the row, not an error object -- a decrypt failure's text
// can carry secret material.
func (s *Service) openProvider(
	ctx context.Context, row *entity.IntegrationSetting,
) entity.ConfiguredProvider {
	provider := entity.ConfiguredProvider{Name: row.Name, Enabled: row.Enabled}

	settings, err := s.resolveAndOpen(ctx, row)
	if err != nil {
		xlog.Error(ctx, "login provider is unreadable",
			xfield.String("kind", row.Kind), xfield.String("name", row.Name),
			xfield.Error(err))

		provider.Unreadable = true

		return provider
	}

	provider.Settings = settings

	return provider
}

// resolveAndOpen finds the implementation this row names and opens its settings.
//
// The implementation is resolved PER ROW, by the row's own name. It used to be
// resolved once for the whole call, which worked while kind named the system --
// and stopped the moment it became a category: the registry keys by name, so
// looking up "login" finds nothing and the call would fail before reading a
// single row, taking sign-in down with it.
func (s *Service) resolveAndOpen(
	ctx context.Context, row *entity.IntegrationSetting,
) (integrationkinds.Settings, error) {
	in, registered := s.registry.lookup(row.Name)
	if !registered {
		// A row whose name nothing implements is unreadable like any other bad
		// row, rather than fatal to the providers that are fine.
		return nil, fmt.Errorf("%w: %q", apperr.ErrUnknownIntegrationKind, row.Name)
	}

	return s.openSettings(ctx, in, row)
}

// openSettings decrypts and parses one row, classifying the failure.
func (s *Service) openSettings(
	ctx context.Context, in integrationkinds.Integration, row *entity.IntegrationSetting,
) (integrationkinds.Settings, error) {
	dek, err := s.unwrapDEKFor(ctx, row.DEKID)
	if err != nil {
		if errors.Is(err, apperr.ErrDataKeyNotFound) || errors.Is(err, apperr.ErrUnwrapDEK) {
			return nil, fmt.Errorf("%w: %w", apperr.ErrIntegrationUnreadable, err)
		}

		// Neither a missing key nor one that will not unwrap: those two carry
		// sentinels and are handled above. What is left is the data_keys READ --
		// the same database the provider rows just came from -- so reaching here
		// says nothing about this row, yet the caller still reports it as
		// unreadable.
		//
		// Left that way deliberately. Unwrapping is local (the KEK lives in this
		// process's config, see crypto.active_kek_uri), so the only way here is a
		// database that answered for integration_settings and then failed for
		// data_keys inside one rebuild -- and a database that is actually down
		// fails the listing first, which keeps the previous snapshot live instead
		// of marking anything. Distinguishing it would mean carrying a typed
		// cause through to Health for a window this narrow.
		//
		// That changes the day active_kek_uri points at a cloud KMS: unwrapping
		// becomes a network call with real transient failures, every provider
		// goes unreadable at once while nothing is wrong with any of them, and
		// the cause has to reach Health rather than only this log line.
		xlog.Error(ctx, "login provider data key could not be read",
			xfield.String("name", row.Name), xfield.Error(err))

		return nil, fmt.Errorf("load dek: %w", err)
	}

	plainSecrets, err := s.decryptAllSecrets(in, row.Config, dek, row.Secrets)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrIntegrationUnreadable, err)
	}

	settings, err := in.Parse(row.Config, plainSecrets)
	if err != nil {
		return nil, fmt.Errorf("%w: parse: %w", apperr.ErrValidation, err)
	}

	// Re-validated on the READ path, not only on write. The field rules are the
	// SSRF guard, the https requirement and the redirect shape, and a row can
	// reach this table by routes that never ran them: a restore from a backup
	// predating the guard, a direct database write, the migration's
	// inter-deploy-step branches. Without this the stored config is trusted
	// because it was once written, which is not the same as being valid now.
	//
	// The failure is ErrValidation, which the reloader already classifies as
	// unresolved -- the provider is listed with its state so an operator can see
	// why, and every other provider in the rebuild is unaffected.
	if err := in.Validate(settings); err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	return settings, nil
}
