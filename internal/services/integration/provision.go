package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// provisionedProvider is one config-declared login provider, parsed: what this
// replica serves, secret included, and the config the row is written from.
type provisionedProvider struct {
	// ConfiguredProvider is served as is by openProvider. Its Settings carry
	// the secret and are never stored.
	entity.ConfiguredProvider
	// Config is the JSON written to the row: the entry's facts and settings as
	// written.
	Config json.RawMessage
}

// Provision writes the config file's login providers into the registry. It runs
// once at startup, before the reloader's first build.
//
// Every entry is validated before anything is written, exactly as a create
// would validate it; one bad entry fails startup and changes no row. Then, in
// one transaction serialized across replicas:
//
//   - a declared provider with no row is inserted, provisioned, with no secret;
//   - a declared provider with a row is rewritten in place, on every start. If
//     that row was the admin API's, this is a TAKE-OVER: the id -- and with it
//     every linked identity -- is kept, and the stored secret is wiped;
//   - a provisioned row the config no longer declares is released: disabled and
//     handed back to the admin API, identities untouched.
//
// The secret is never written. This replica serves provisioned providers from
// memory (see ListLoginProviders), so the database holds only the row.
//
// Any error fails startup; there is no retry in-process. A replica restarted by
// the orchestrator provisions again from the top.
//
// Call it exactly once per process, before the login reloader starts: it sets
// the in-memory providers the reloader reads without a lock, which is safe only
// because nothing reads them before this returns and nothing writes them after.
func (s *Service) Provision(ctx context.Context) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.Provision")
	defer span.End()

	declared, err := s.parseDeclared()
	if err != nil {
		return err
	}

	err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		if lockErr := dbtx.AdvisoryXactLock(ctx, dbtx.AdvisoryLockKeyLoginProvisioning); lockErr != nil {
			return fmt.Errorf("acquire login provisioning lock: %w", lockErr)
		}

		for name, provider := range declared {
			if provisionErr := s.provisionOne(ctx, name, provider); provisionErr != nil {
				return fmt.Errorf("provision login provider %q: %w", name, provisionErr)
			}
		}

		return s.releaseUndeclared(ctx, declared)
	})
	if err != nil {
		return fmt.Errorf("provision login providers: %w", err)
	}

	s.provisioned = declared

	return nil
}

// parseDeclared parses every entry with managed_by: config into what Provision
// writes and serves; an entry that does not parse is an error, so the create
// rules are enforced here. Entries managed by the UI are skipped: they declare
// nothing here. Errors name the provider; none carries a secret value.
func (s *Service) parseDeclared() (map[string]provisionedProvider, error) {
	declared := map[string]provisionedProvider{}

	var errs error
	for name, entry := range s.loginProviders {
		if entry.ManagedBy != config.ManagedByConfig {
			continue
		}

		provider, err := s.parseDeclaredEntry(name, entry)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("provisioned login provider %q: %w", name, err))
			continue
		}
		declared[name] = provider
	}

	return declared, errs
}

// parseDeclaredEntry parses one declared entry, holding it to the same rules as
// a create.
//
// Its config is used AS WRITTEN: the facts and the settings of the same entry,
// with no preset injected or refused. That is an accepted trust decision -- the
// author of this file already controls the catalog the preset rules read, so
// pointing `google` elsewhere from here is an operator's act, not an attack.
// The preset rules guard the admin API, which is where the line is.
func (s *Service) parseDeclaredEntry(name string, entry config.LoginProvider) (provisionedProvider, error) {
	if err := s.registry.admit(integrationkinds.CategoryLogin, name); err != nil {
		return provisionedProvider{}, err
	}

	in, err := s.registry.get(name)
	if err != nil {
		return provisionedProvider{}, err
	}

	cfg, secrets, err := declaredSettings(in, entry)
	if err != nil {
		return provisionedProvider{}, err
	}

	// Pinned off: never served, so there is nothing to validate the settings
	// for, and demanding credentials for a switched-off provider would push an
	// operator towards handing it to the UI just to switch it off.
	//
	// Enabled is never nil here: the config loader refuses a managed_by: config
	// entry without it (config.checkMode).
	if !lo.FromPtr(entry.Enabled) {
		return provisionedProvider{
			ConfiguredProvider: entity.ConfiguredProvider{Name: name},
			Config:             cfg,
		}, nil
	}

	_, settings, err := s.resolveSettings(name, cfg, secrets)
	if err != nil {
		return provisionedProvider{}, err
	}

	return provisionedProvider{
		ConfiguredProvider: entity.ConfiguredProvider{Name: name, Enabled: true, Settings: settings},
		Config:             cfg,
	}, nil
}

// declaredSettings turns an entry into the row's config and the provider's
// secrets. The config loader already split the entry by value -- references in
// Secrets, everything else in Settings -- and the provider's kind says which
// keys are secret, so only those are taken from Secrets:
//
//   - a secret written literally is refused, since this file is the one that
//     gets committed; checked whether or not the provider is enabled, as a
//     pinned-off entry would otherwise write it into the row. The error names
//     the key, never the value;
//   - a missing secret is left to the kind's Validate, which a pinned-off
//     entry never reaches -- it needs no credentials;
//   - a reference under a key the kind does not treat as secret is ignored.
func declaredSettings(
	in integrationkinds.Integration, entry config.LoginProvider,
) (json.RawMessage, map[string]string, error) {
	secrets := map[string]string{}
	for _, key := range in.SecretKeys() {
		if _, literal := entry.Settings[key]; literal {
			return nil, nil, fmt.Errorf(
				"%w: %s must be a <secret:KEY> reference to the secrets file, not a literal value",
				apperr.ErrValidation, key)
		}
		if value, ok := entry.Secrets[key]; ok {
			secrets[key] = value
		}
	}

	fields := map[string]any{}
	maps.Copy(fields, entry.Settings)
	// The facts come as map[string]string, which maps.Copy will not put into a
	// map[string]any.
	for key, value := range entry.Fields() {
		fields[key] = value
	}

	cfg, err := json.Marshal(fields)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: config: %w", apperr.ErrValidation, err)
	}

	return cfg, secrets, nil
}

// provisionOne inserts or rewrites one declared provider.
//
// An existing row is rewritten unconditionally, secret wiped either way: a
// restart with an unchanged config costs one UPDATE of a handful of rows, which
// is cheaper than working out whether anything moved.
func (s *Service) provisionOne(ctx context.Context, name string, provider provisionedProvider) error {
	existing, err := s.store.GetForUpdateByKindName(ctx, integrationkinds.CategoryLogin, name)
	if errors.Is(err, apperr.ErrIntegrationNotFound) {
		return s.insertProvisioned(ctx, name, provider)
	}
	if err != nil {
		return err
	}

	if !existing.Provisioned {
		xlog.Warn(ctx, "login provider taken over by the config file; its stored secret was wiped",
			xfield.String("name", name))
	}

	existing.Enabled = provider.Enabled
	existing.Config = provider.Config
	existing.Secrets = nil
	existing.Provisioned = true
	existing.UpdatedByUserID = nil

	_, err = s.store.Update(ctx, existing)

	return err
}

// insertProvisioned creates the row for a provider no row exists for yet. It
// still gets a DEK of its own: nothing is sealed under it now, but a released
// row becomes an ordinary one an admin can put a secret into.
func (s *Service) insertProvisioned(ctx context.Context, name string, provider provisionedProvider) error {
	_, dekID, err := s.newDEK(ctx)
	if err != nil {
		return err
	}

	_, err = s.store.Create(ctx, &entity.IntegrationSetting{
		Kind:        integrationkinds.CategoryLogin,
		Name:        name,
		Enabled:     provider.Enabled,
		Config:      provider.Config,
		DEKID:       dekID,
		Provisioned: true,
	})

	return err
}

// releaseUndeclared hands back to the UI every provisioned row this config no
// longer declares -- its entry was removed, or switched to managed_by: ui.
func (s *Service) releaseUndeclared(ctx context.Context, declared map[string]provisionedProvider) error {
	rows, err := s.store.ListByKind(ctx, integrationkinds.CategoryLogin)
	if err != nil {
		return fmt.Errorf("list login providers: %w", err)
	}

	var gone []string
	for _, row := range rows {
		if _, ok := declared[row.Name]; row.Provisioned && !ok {
			gone = append(gone, row.Name)
		}
	}

	if err := s.store.ReleaseProvisioned(ctx, integrationkinds.CategoryLogin, gone); err != nil {
		return fmt.Errorf("release login providers: %w", err)
	}

	if len(gone) > 0 {
		xlog.Warn(ctx, "login providers no longer declared in the config file; released to the UI and disabled",
			xfield.Strings("names", gone))
	}

	return nil
}
