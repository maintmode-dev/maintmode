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
// A missing or disabled integration returns ErrIntegrationDisabled, which the
// dispatch path treats as a best-effort drop (not an error). The decrypted
// secrets live inside the returned Settings; the caller (the transport builder)
// captures what it needs and drops them — Settings types carry no
// Stringer/marshaler, so they cannot be logged wholesale by accident.
func (s *Service) Settings(ctx context.Context, kind string) (integrationkinds.Settings, error) {
	setting, err := s.store.GetByKind(ctx, kind)
	if errors.Is(err, apperr.ErrIntegrationNotFound) {
		return nil, fmt.Errorf("%w: %q not configured", apperr.ErrIntegrationDisabled, kind)
	}
	if err != nil {
		return nil, fmt.Errorf("settings %q: %w", kind, err)
	}
	if !setting.Enabled {
		return nil, apperr.ErrIntegrationDisabled
	}

	in, err := s.registry.Get(kind)
	if err != nil {
		return nil, err
	}

	dek, err := s.unwrapDEKFor(ctx, setting.DEKID)
	if err != nil {
		return nil, err
	}

	plainSecrets, err := s.decryptAllSecrets(in, dek, setting.Secrets)
	if err != nil {
		return nil, err
	}

	parsed, err := in.Parse(setting.Config, plainSecrets)
	if err != nil {
		return nil, fmt.Errorf("settings %q: parse: %w", kind, err)
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
