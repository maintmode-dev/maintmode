package integration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// Settings returns the typed, decrypted settings of an ENABLED integration —
// the registry's read seam for the delivery side (services/transportresolver
// consumes it to build transport clients). The registry itself knows nothing
// about transports; this is as far as it goes.
//
// Lookup is by (kind, name) (matching messenger_channels.transport) and
// deliberately does NOT gate on entity.NotifyTransport.IsValid(): "email" is a
// valid integration kind even though it is not a user-subscribable channel.
//
// A missing integration returns ErrIntegrationNotConfigured and a disabled one
// ErrIntegrationDisabled; the former wraps the latter, so the dispatch path's
// errors.Is(err, ErrIntegrationDisabled) best-effort drop covers both, while
// the read model can still tell them apart. An ENABLED integration
// whose secrets cannot be opened locally (rolled-back KEK, corrupt envelope,
// missing DEK row, unparseable settings) returns ErrIntegrationUnreadable —
// surfaced as transport_status "unreadable" on reads, retried toward
// dead-letter on delivery. A plain storage failure stays unwrapped: it is an
// infrastructure error, not an integration state.
//
// An integration this replica's config declares is served from memory, with
// no database read -- see servedFromConfig.
//
// The decrypted secrets live inside the returned Settings; the caller (the
// transport builder) captures what it needs and drops them — Settings types
// carry no Stringer/marshaler, so they cannot be logged wholesale by accident.
func (s *Service) Settings(ctx context.Context, kind, name string) (integrationkinds.Settings, error) {
	if settings, declared, err := s.servedFromConfig(kind, name); declared {
		return settings, err
	}

	setting, err := s.store.GetByKindName(ctx, kind, name)
	if errors.Is(err, apperr.ErrIntegrationNotFound) {
		return nil, fmt.Errorf("%w: %q", apperr.ErrIntegrationNotConfigured, name)
	}
	if err != nil {
		return nil, fmt.Errorf("settings %q: %w", name, err)
	}
	if !setting.Enabled {
		return nil, apperr.ErrIntegrationDisabled
	}

	// Provisioned, but not by this replica's config: another replica declared
	// it, and its secret lives only in that replica's memory. Opening the row
	// would find no secret and -- since nothing here validates -- hand back
	// settings with an empty token, which builds a transport that reports
	// healthy and fails every send. A replica on a different config is the
	// cause, so it is named in the log.
	if setting.Provisioned {
		xlog.Warn(ctx, "integration is provisioned by a config this replica does not share; not serving it",
			xfield.String("category", kind), xfield.String("name", name))

		return nil, fmt.Errorf("%w: %q is declared in another replica's config", apperr.ErrIntegrationUnreadable, name)
	}

	// From here on the integration is enabled and every failure is a local
	// can't-open-it condition, classified unreadable.
	in, err := s.registry.get(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %q not in registry: %w", apperr.ErrIntegrationUnreadable, name, err)
	}

	dek, err := s.unwrapDEKFor(ctx, setting.DEKID)
	if err != nil {
		// A missing DEK row or a failed unwrap are known can't-open-it
		// conditions → unreadable. Anything else (e.g. a DB outage on the load)
		// stays unwrapped so it surfaces as a 500, not a channel status.
		if errors.Is(err, apperr.ErrDataKeyNotFound) || errors.Is(err, apperr.ErrUnwrapDEK) {
			return nil, fmt.Errorf("%w: %w", apperr.ErrIntegrationUnreadable, err)
		}
		return nil, fmt.Errorf("settings %q: load dek: %w", kind, err)
	}

	plainSecrets, err := s.decryptAllSecrets(in, setting.Config, dek, setting.Secrets)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrIntegrationUnreadable, err)
	}

	parsed, err := in.Parse(setting.Config, plainSecrets)
	if err != nil {
		return nil, fmt.Errorf("%w: parse: %w", apperr.ErrIntegrationUnreadable, err)
	}
	return parsed, nil
}

// decryptAllSecrets opens every stored secret the kind declares, returning the
// plaintext map for Parse. A missing key is skipped (the kind's Validate decides
// whether it was required).
//
// config is parsed first, WITHOUT secrets, to recover the AAD binding. That is
// not circular: the fields a login provider binds to (issuer_url, client_id)
// live in the plaintext config column, precisely so opening a secret never
// needs another secret. It must be the same binding the seal path computed, so
// both go through secretAAD rather than reaching for an AAD function directly.
func (s *Service) decryptAllSecrets(
	in integrationkinds.Integration, config json.RawMessage, dek []byte, stored map[string]string,
) (map[string]string, error) {
	bindingSettings, err := in.Parse(config, nil)
	if err != nil {
		return nil, fmt.Errorf("parse config for secret binding: %w", err)
	}

	plain := make(map[string]string, len(stored))
	for _, key := range in.SecretKeys() {
		enc, ok := stored[key]
		if !ok {
			continue
		}
		envelope, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("decode secret %q: %w", key, err)
		}
		value, err := s.cipher.Decrypt(dek, envelope, secretAAD(in, bindingSettings, key))
		if err != nil {
			return nil, fmt.Errorf("decrypt secret %q: %w", key, err)
		}
		plain[key] = string(value)
	}
	return plain, nil
}

// servedFromConfig answers for an integration this replica's config declares:
// its parsed settings when enabled, ErrIntegrationDisabled when pinned off. The
// bool is false for anything the config does not declare, which the caller then
// reads from its row.
//
// The row is not consulted at all, the same way openProvider serves a login
// provider: it is a display copy written by whichever replica started last, and
// during a rolling config change it may carry another replica's settings.
func (s *Service) servedFromConfig(
	kind integrationkinds.Category, name string,
) (integrationkinds.Settings, bool, error) {
	declared, ok := s.provisioned[kind][name]
	if !ok {
		return nil, false, nil
	}

	if !declared.Enabled {
		return nil, true, apperr.ErrIntegrationDisabled
	}

	return declared.Settings, true, nil
}
