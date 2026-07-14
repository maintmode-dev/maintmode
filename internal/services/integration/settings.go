package integration

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
)

// Settings returns the typed, decrypted settings of an ENABLED integration —
// the registry's read seam for the delivery side (services/transportresolver
// consumes it to build transport clients). The registry itself knows nothing
// about transports; this is as far as it goes.
//
// Lookup is by the raw kind string (matching messenger_channels.transport) and
// deliberately does NOT gate on entity.NotifyTransport.IsValid(): "email" is a
// valid integration kind even though it is not a user-subscribable channel.
//
// A missing integration returns ErrIntegrationNotConfigured and a disabled one
// ErrIntegrationDisabled; the former wraps the latter, so the dispatch path's
// errors.Is(err, ErrIntegrationDisabled) best-effort drop covers both, while
// the read model can still tell them apart (RUK-200). An ENABLED integration
// whose secrets cannot be opened locally (rolled-back KEK, corrupt envelope,
// missing DEK row, unparseable settings) returns ErrIntegrationUnreadable —
// surfaced as transport_status "unreadable" on reads, retried toward
// dead-letter on delivery. A plain storage failure stays unwrapped: it is an
// infrastructure error, not an integration state.
//
// The decrypted secrets live inside the returned Settings; the caller (the
// transport builder) captures what it needs and drops them — Settings types
// carry no Stringer/marshaler, so they cannot be logged wholesale by accident.
func (s *Service) Settings(ctx context.Context, kind string) (integrationkinds.Settings, error) {
	setting, err := s.store.GetByKind(ctx, kind)
	if errors.Is(err, apperr.ErrIntegrationNotFound) {
		return nil, fmt.Errorf("%w: %q", apperr.ErrIntegrationNotConfigured, kind)
	}
	if err != nil {
		return nil, fmt.Errorf("settings %q: %w", kind, err)
	}
	if !setting.Enabled {
		return nil, apperr.ErrIntegrationDisabled
	}

	// From here on the integration is enabled and every failure is a local
	// can't-open-it condition, classified unreadable.
	in, err := s.registry.Get(kind)
	if err != nil {
		return nil, fmt.Errorf("%w: kind %q not in registry: %w", apperr.ErrIntegrationUnreadable, kind, err)
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

	plainSecrets, err := s.decryptAllSecrets(in, dek, setting.Secrets)
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
func (s *Service) decryptAllSecrets(in integrationkinds.Integration, dek []byte, stored map[string]string) (map[string]string, error) {
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
		value, err := s.cipher.Decrypt(dek, envelope, secrets.SecretAAD(in.Kind(), key))
		if err != nil {
			return nil, fmt.Errorf("decrypt secret %q: %w", key, err)
		}
		plain[key] = string(value)
	}
	return plain, nil
}
