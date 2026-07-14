package integration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
)

// validateKind parses and validates the config+secrets for a kind. It returns
// the registered Integration so callers can reuse its SecretKeys.
func (s *Service) validateKind(kind string, config json.RawMessage, plainSecrets map[string]string) (integrationkinds.Integration, error) {
	in, err := s.registry.Get(kind)
	if err != nil {
		return nil, err
	}
	if err := rejectSecretKeysInConfig(in, config); err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}
	settings, err := in.Parse(config, plainSecrets)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}
	if err := in.Validate(settings); err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}
	return in, nil
}

// rejectSecretKeysInConfig guards against the most likely section mix-up: a
// credential submitted under its well-known key inside the plaintext config
// instead of secrets. Config is stored and returned verbatim, so such a value
// would sit unencrypted in the DB and leak in every GET. Only an exact
// SecretKeys() name match is detectable — a secret under an arbitrary key
// cannot be classified and stays the caller's responsibility.
func rejectSecretKeysInConfig(in integrationkinds.Integration, config json.RawMessage) error {
	if len(config) == 0 {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(config, &fields); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	for _, key := range in.SecretKeys() {
		if _, ok := fields[key]; ok {
			return fmt.Errorf("config key %q is a secret and belongs in secrets", key)
		}
	}
	return nil
}

// warnUnknownSecretKeys logs any incoming secret key the kind does not declare.
// Unknown keys are ignored, never stored — so a typo ("pasword") silently loses
// the credential; this warning is the diagnostic trail. Key NAMES only, never
// values.
func warnUnknownSecretKeys[V any](ctx context.Context, in integrationkinds.Integration, incoming map[string]V) {
	known := make(map[string]struct{}, len(in.SecretKeys()))
	for _, key := range in.SecretKeys() {
		known[key] = struct{}{}
	}

	var unknown []string
	for key := range incoming {
		if _, ok := known[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return
	}
	sort.Strings(unknown)
	xlog.Warn(ctx, "ignoring unknown secret keys for integration",
		xfield.String("kind", in.Kind()),
		xfield.Strings("secret_keys", unknown),
	)
}

// newDEK generates a fresh DEK, wraps it with the active KEK, and stores the
// wrapped copy — returning the plaintext DEK plus the data_keys row id. Used by
// Create; Update reuses the existing DEK via unwrapDEKFor. Must run in a tx.
func (s *Service) newDEK(ctx context.Context) (dek []byte, dekID uuid.UUID, err error) {
	dek, err = secrets.GenerateDEK()
	if err != nil {
		return nil, uuid.Nil, err
	}
	wrapped, kekID, err := s.keyring.WrapDEK(dek)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("wrap new dek: %w", err)
	}
	row, err := s.dekStore.Create(ctx, &entity.DataKey{KEKID: kekID, EncryptedDEK: wrapped})
	if err != nil {
		return nil, uuid.Nil, err
	}
	return dek, row.ID, nil
}

// encryptSecrets seals each plaintext secret value under dek and returns the
// stored secrets map ({key: base64(envelope)}) ready to persist — the store
// mapper marshals it to jsonb at the DB boundary. Only keys the kind declares
// secret are considered; an unknown key is ignored.
func (s *Service) encryptSecrets(in integrationkinds.Integration, dek []byte, plain map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(plain))
	for _, key := range in.SecretKeys() {
		v, ok := plain[key]
		if !ok || v == "" {
			continue // absent secret: nothing to store for this key
		}
		sealed, err := s.sealSecret(in, dek, key, v)
		if err != nil {
			return nil, err
		}
		out[key] = sealed
	}
	return out, nil
}

// sealSecret encrypts one plaintext secret value under dek, bound to its
// (kind, key) slot via AAD, and returns the storable base64(envelope) form.
// The single seal path — create and update both go through here, so the AAD
// binding and encoding can never drift apart between them.
func (s *Service) sealSecret(in integrationkinds.Integration, dek []byte, key, value string) (string, error) {
	envelope, err := s.cipher.Encrypt(dek, []byte(value), secrets.SecretAAD(in.Kind(), key))
	if err != nil {
		return "", fmt.Errorf("encrypt secret %q: %w", key, err)
	}
	return base64.StdEncoding.EncodeToString(envelope), nil
}

// mergedSecrets is the result of merging incoming plaintext over stored
// ciphertext: stored is the final key->ciphertext map to persist (the store
// mapper marshals it at the DB boundary), and view is the plaintext-ish map
// for kind re-validation — changed keys carry their real plaintext, carried-over keys a
// non-empty sentinel, so Validate sees them as present without the service
// decrypting an unchanged secret.
type mergedSecrets struct {
	stored map[string]string
	view   map[string]string
}

// secretPresentSentinel stands in for an unchanged secret during re-validation so
// the value need not be decrypted. It is never persisted.
const secretPresentSentinel = "\x00present"

// mergeSecrets overlays the incoming plaintext over the stored ciphertext: a key
// present in incoming is re-encrypted under the setting's DEK; a key absent keeps
// its stored ciphertext verbatim (no decrypt); a null/empty value clears it
// (dropped, not stored). The DEK is unwrapped only when at least one
// secret actually changes, so a config-only update never materializes any
// secret plaintext.
func (s *Service) mergeSecrets(
	ctx context.Context,
	in integrationkinds.Integration,
	current *entity.IntegrationSetting,
	incoming map[string]*string,
) (*mergedSecrets, error) {
	stored := make(map[string]string)
	view := make(map[string]string)

	// dekOnce unwraps the DEK at most once, and only if a secret actually changes.
	dekOnce := s.lazyDEK(ctx, current.DEKID)

	for _, key := range in.SecretKeys() {
		intent, sent := incoming[key]
		switch {
		case !sent:
			// KEEP: carry the stored ciphertext forward as-is (no decrypt); the
			// view marks the key present via the sentinel.
			if enc, ok := current.Secrets[key]; ok {
				stored[key] = enc
				view[key] = secretPresentSentinel
			}
		case intent == nil || *intent == "":
			// CLEAR (null or ""): the key simply does not make it into the result.
		default:
			// REPLACE: encrypt the new value under the setting's existing DEK.
			dek, err := dekOnce()
			if err != nil {
				return nil, err
			}
			sealed, err := s.sealSecret(in, dek, key, *intent)
			if err != nil {
				return nil, err
			}
			stored[key] = sealed
			view[key] = *intent
		}
	}

	return &mergedSecrets{stored: stored, view: view}, nil
}

// lazyDEK returns a memoizing accessor that unwraps the setting's DEK on first
// call and caches it (value AND error), so a merge that changes no secret never
// touches the DEK.
func (s *Service) lazyDEK(ctx context.Context, dekID uuid.UUID) func() ([]byte, error) {
	return sync.OnceValues(func() ([]byte, error) {
		return s.unwrapDEKFor(ctx, dekID)
	})
}

// unwrapDEKFor loads and unwraps the DEK for an existing setting's data_keys row.
func (s *Service) unwrapDEKFor(ctx context.Context, dekID uuid.UUID) ([]byte, error) {
	row, err := s.dekStore.GetByID(ctx, dekID)
	if err != nil {
		return nil, fmt.Errorf("get dek: %w", err)
	}
	dek, err := s.keyring.UnwrapDEK(row.EncryptedDEK, row.KEKID)
	if err != nil {
		return nil, fmt.Errorf("%w: unwrap dek: %w", apperr.ErrUnwrapDEK, err)
	}
	return dek, nil
}
