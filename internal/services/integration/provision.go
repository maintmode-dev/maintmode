package integration

import (
	"bytes"
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

// provisionedCategories are the categories the config file can declare
// integrations in, in the order Provision writes them.
var provisionedCategories = []integrationkinds.Category{integrationkinds.CategoryLogin, integrationkinds.CategoryNotify}

// declaration is one config entry as Provision reads it, whatever its category:
// its name, the entry, and the facts a login provider carries beside it (nil
// for any other category).
type declaration struct {
	category integrationkinds.Category
	name     string
	entry    config.ManagedEntry
	facts    map[string]string
}

// provisionedEntry is one config-declared integration, parsed: what this
// replica serves, secret included, and the config the row is written from.
type provisionedEntry struct {
	Enabled bool
	// Settings is served as is, secret included, and never stored. Nil for an
	// entry pinned off.
	Settings integrationkinds.Settings
	// Config is the JSON written to the row: the entry's facts and settings as
	// written.
	Config json.RawMessage
}

// Provision writes the config file's declared integrations into the registry.
// It runs once at startup, before the reloader's first build.
//
// Every entry is validated before anything is written, exactly as a create
// would validate it; one bad entry fails startup and changes no row. Then, in
// one transaction serialized across replicas, per category:
//
//   - a declared integration with no row is inserted, provisioned, with no
//     secret;
//   - a declared integration with a row is rewritten in place, on every start.
//     If that row was the admin API's, this is a TAKE-OVER: the id -- and with
//     it every linked identity -- is kept, and the stored secret is wiped;
//   - a provisioned row the config no longer declares is released: disabled
//     and handed back to the admin API, identities untouched.
//
// The secret is never written. This replica serves provisioned integrations
// from memory -- login providers through openProvider, notify transports
// through Settings -- so the database holds only the row.
//
// Any error fails startup; there is no retry in-process. A replica restarted by
// the orchestrator provisions again from the top.
//
// Call it exactly once per process, before the login reloader starts: it sets
// the in-memory entries the reloader reads without a lock, which is safe only
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
			return fmt.Errorf("acquire provisioning lock: %w", lockErr)
		}

		for _, category := range provisionedCategories {
			if provisionErr := s.provisionCategory(ctx, category, declared[category]); provisionErr != nil {
				return provisionErr
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.provisioned = declared

	return nil
}

// provisionCategory writes one category's declared entries, keyed by name,
// and releases its undeclared ones.
func (s *Service) provisionCategory(
	ctx context.Context, category integrationkinds.Category, declared map[string]provisionedEntry,
) error {
	for name, entry := range declared {
		if err := s.provisionOne(ctx, category, name, entry); err != nil {
			return fmt.Errorf("provision %s/%s: %w", category, name, err)
		}
	}

	return s.releaseUndeclared(ctx, category, declared)
}

// declarations is every entry of every section the config file declares
// integrations in.
//
// A list rather than a map by name: the two sections are keyed separately, so
// a transport and a provider can share a name, and neither may silently shadow
// the other. A declared one under the wrong category is refused by admit.
func (s *Service) declarations() []declaration {
	out := make([]declaration, 0, len(s.loginProviders)+len(s.notifyTransports))
	for name, provider := range s.loginProviders {
		out = append(out, declaration{
			category: integrationkinds.CategoryLogin,
			name:     name,
			entry:    provider.ManagedEntry,
			facts:    provider.Fields(),
		})
	}
	for name, transport := range s.notifyTransports {
		out = append(out, declaration{category: integrationkinds.CategoryNotify, name: name, entry: transport})
	}

	return out
}

// parseDeclared parses every entry with managed_by: config into what Provision
// writes and serves; an entry that does not parse is an error, so the create
// rules are enforced here. Entries managed by the UI are skipped: they declare
// nothing here. Errors name the entry; none carries a secret value.
//
// The result is keyed by category and then by name, the (kind, name) a row is
// addressed by.
func (s *Service) parseDeclared() (map[integrationkinds.Category]map[string]provisionedEntry, error) {
	declared := map[integrationkinds.Category]map[string]provisionedEntry{}

	var errs error
	for _, decl := range s.declarations() {
		if decl.entry.ManagedBy != config.ManagedByConfig {
			continue
		}

		entry, err := s.parseDeclaredEntry(decl)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("provisioned %s/%s: %w", decl.category, decl.name, err))
			continue
		}
		if declared[decl.category] == nil {
			declared[decl.category] = map[string]provisionedEntry{}
		}
		declared[decl.category][decl.name] = entry
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
func (s *Service) parseDeclaredEntry(decl declaration) (provisionedEntry, error) {
	if err := s.registry.admit(decl.category, decl.name); err != nil {
		return provisionedEntry{}, err
	}

	in, err := s.registry.get(decl.name)
	if err != nil {
		return provisionedEntry{}, err
	}

	cfg, secrets, err := declaredSettings(in, decl)
	if err != nil {
		return provisionedEntry{}, err
	}

	// Pinned off: never served, so there is nothing to validate the settings
	// for, and demanding credentials for a switched-off integration would push
	// an operator towards handing it to the UI just to switch it off.
	//
	// Enabled is never nil here: the config loader refuses a managed_by: config
	// entry without it.
	if !lo.FromPtr(decl.entry.Enabled) {
		return provisionedEntry{Config: cfg}, nil
	}

	_, settings, err := s.resolveSettings(decl.name, cfg, secrets)
	if err != nil {
		return provisionedEntry{}, err
	}

	return provisionedEntry{Enabled: true, Settings: settings, Config: cfg}, nil
}

// declaredSettings turns an entry into the row's config and the integration's
// secrets. The config loader already split the entry by value -- references in
// Secrets, everything else in Settings -- and the integration's kind says which
// keys are secret, so only those are taken from Secrets:
//
//   - a secret written literally is refused, since this file is the one that
//     gets committed; checked whether or not the entry is enabled, as a
//     pinned-off entry would otherwise write it into the row. The error names
//     the key, never the value;
//   - a missing secret is left to the kind's Validate, which a pinned-off
//     entry never reaches -- it needs no credentials;
//   - a reference under a key the kind does not treat as secret is ignored;
//   - a key the kind does not read at all is refused -- see refuseUnknownKeys.
func declaredSettings(
	in integrationkinds.Integration, decl declaration,
) (json.RawMessage, map[string]string, error) {
	secrets := map[string]string{}
	for _, key := range in.SecretKeys() {
		if _, literal := decl.entry.Settings[key]; literal {
			return nil, nil, fmt.Errorf(
				"%w: %s must be a <secret:KEY> reference to the secrets file, not a literal value",
				apperr.ErrValidation, key)
		}
		if value, ok := decl.entry.Secrets[key]; ok {
			secrets[key] = value
		}
	}

	fields := map[string]any{}
	maps.Copy(fields, decl.entry.Settings)
	// The facts come as map[string]string, which maps.Copy will not put into a
	// map[string]any.
	for key, value := range decl.facts {
		fields[key] = value
	}

	cfg, err := json.Marshal(fields)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: config: %w", apperr.ErrValidation, err)
	}

	if err := refuseUnknownKeys(in, cfg); err != nil {
		return nil, nil, err
	}

	return cfg, secrets, nil
}

// refuseUnknownKeys holds a declared config to the shape of its kind's
// settings: a key the kind does not read, or a value of the wrong type, fails
// startup.
//
// Stricter than the admin API on purpose. Everything in the config is written
// into the row's PLAINTEXT config column and returned by the admin read model,
// so a secret under a name the kind does not know -- `token` for `bot_token`, a
// typo, another tool's spelling -- would be stored unencrypted and shown to
// every reader, with nothing to say it was ignored. The check runs whether or
// not the entry is enabled, since a pinned-off entry's config is written too.
//
// The error is encoding/json's, which names the key and the Go type, never a
// string value. A secret key itself never reaches here as a known field -- its
// Settings field is json:"-" -- and a literal one was refused above.
func refuseUnknownKeys(in integrationkinds.Integration, cfg json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(cfg))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(in.EmptySettings()); err != nil {
		return fmt.Errorf("%w: config: %w", apperr.ErrValidation, err)
	}

	return nil
}

// provisionOne inserts or rewrites one declared integration.
//
// An existing row is rewritten unconditionally, secret wiped either way: a
// restart with an unchanged config costs one UPDATE of a handful of rows, which
// is cheaper than working out whether anything moved.
func (s *Service) provisionOne(
	ctx context.Context, category integrationkinds.Category, name string, entry provisionedEntry,
) error {
	existing, err := s.store.GetForUpdateByKindName(ctx, category, name)
	if errors.Is(err, apperr.ErrIntegrationNotFound) {
		return s.insertProvisioned(ctx, category, name, entry)
	}
	if err != nil {
		return err
	}

	if !existing.Provisioned {
		xlog.Warn(ctx, "integration taken over by the config file; its stored secret was wiped",
			xfield.String("category", category), xfield.String("name", name))
	}

	existing.Enabled = entry.Enabled
	existing.Config = entry.Config
	existing.Secrets = nil
	existing.Provisioned = true
	existing.UpdatedByUserID = nil

	_, err = s.store.Update(ctx, existing)

	return err
}

// insertProvisioned creates the row for an integration no row exists for yet.
// It still gets a DEK of its own: nothing is sealed under it now, but a
// released row becomes an ordinary one an admin can put a secret into.
func (s *Service) insertProvisioned(
	ctx context.Context, category integrationkinds.Category, name string, entry provisionedEntry,
) error {
	_, dekID, err := s.newDEK(ctx)
	if err != nil {
		return err
	}

	_, err = s.store.Create(ctx, &entity.IntegrationSetting{
		Kind:        category,
		Name:        name,
		Enabled:     entry.Enabled,
		Config:      entry.Config,
		DEKID:       dekID,
		Provisioned: true,
	})

	return err
}

// releaseUndeclared hands back to the UI every provisioned row of a category
// this config no longer declares -- its entry was removed, or switched to
// managed_by: ui. declared is the category's entries, keyed by name.
//
// Only rows whose name this service's registry holds under the category are
// released. In production that is every name, so nothing changes; in a test
// registering its own renamed kinds, it keeps the run off the rows a stand
// provisioned on the same database.
func (s *Service) releaseUndeclared(
	ctx context.Context, category integrationkinds.Category, declared map[string]provisionedEntry,
) error {
	rows, err := s.store.ListByKind(ctx, category)
	if err != nil {
		return fmt.Errorf("list %s integrations: %w", category, err)
	}

	var gone []string
	for _, row := range rows {
		// An admin's row: not the config's to release.
		if !row.Provisioned {
			continue
		}
		// Still declared: rewritten above, stays provisioned.
		if _, ok := declared[row.Name]; ok {
			continue
		}
		// A name this registry does not hold under the category, such as a
		// stand's row seen by a test with renamed kinds.
		if s.registry.admit(category, row.Name) != nil {
			continue
		}

		gone = append(gone, row.Name)
	}

	if err := s.store.ReleaseProvisioned(ctx, category, gone); err != nil {
		return fmt.Errorf("release %s integrations: %w", category, err)
	}

	if len(gone) > 0 {
		xlog.Warn(ctx, "integrations no longer declared in the config file; released to the UI and disabled",
			xfield.String("category", category), xfield.Strings("names", gone))
	}

	return nil
}
