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
	"github.com/ruko1202/maintmode/internal/utils/xurl"
)

// resolveSettings turns a row's raw config and secrets into the values a write
// needs, refusing the write if any step says no.
//
// Named for what it PRODUCES, not for the refusal: it returns the registered
// Integration, whose SecretKeys and Name the caller reuses, and the parsed
// Settings, which the secret path needs to choose the AAD binding. Returning
// the parsed value rather than re-parsing keeps one parse per write, so the
// bytes validated and the bytes bound can never be two different things -- and
// that is the reason the function exists at all, so a name saying only
// "validate" describes the half a caller can ignore.
//
// One caller does ignore it: Update re-runs this on the merged state purely to
// find out whether the result would still be valid, and drops both values. That
// is a legitimate second use of the same work, not the primary one.
//
// It takes the NAME, because that is what the registry keys on. The category is
// checked separately, by the registry's admit: a row is legal only when both halves match
// a registered entry, and checking the name alone would admit (notify, google).
func (s *Service) resolveSettings(
	name string, config json.RawMessage, plainSecrets map[string]string,
) (integrationkinds.Integration, integrationkinds.Settings, error) {
	in, err := s.registry.get(name)
	if err != nil {
		return nil, nil, err
	}
	if err := rejectSecretKeysInConfig(in, config); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}
	settings, err := in.Parse(config, plainSecrets)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}
	if err := in.Validate(settings); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}
	return in, settings, nil
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
	unknown := unknownSecretKeys(in, incoming)
	if len(unknown) == 0 {
		return
	}
	xlog.Warn(ctx, "ignoring unknown secret keys for integration",
		xfield.String("name", in.Name()),
		xfield.Strings("secret_keys", unknown),
	)
}

// unknownSecretKeys returns, sorted, the incoming secret keys the kind does not
// declare. Callers differ on what that means — create/update drop them with a
// warning, the probe refuses outright — so this only names them.
func unknownSecretKeys[V any](in integrationkinds.Integration, incoming map[string]V) []string {
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
	sort.Strings(unknown)
	return unknown
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
func (s *Service) encryptSecrets(
	in integrationkinds.Integration, settings integrationkinds.Settings, dek []byte, plain map[string]string,
) (map[string]string, error) {
	out := make(map[string]string, len(plain))
	for _, key := range in.SecretKeys() {
		v, ok := plain[key]
		if !ok || v == "" {
			continue // absent secret: nothing to store for this key
		}
		sealed, err := s.sealSecret(in, settings, dek, key, v)
		if err != nil {
			return nil, err
		}
		out[key] = sealed
	}
	return out, nil
}

// secretAAD picks the binding for one secret slot.
//
// A kind that declares itself ClientBound (the login providers) binds its
// secrets to the OAuth client they were issued for as well as to the slot;
// everything else uses the plain (kind, key) form that every stored
// Slack/SMTP/Telegram secret is already sealed under.
//
// settings is the EFFECTIVE state being written — on update that means the
// post-patch config, not what is currently stored. Sealing under the stored
// values while persisting new ones is the way to produce a row that can never
// be decrypted again, so the caller passes what it is about to save.
//
// The single AAD path: seal and open both come through here, so the two can
// never drift apart.
func secretAAD(in integrationkinds.Integration, settings integrationkinds.Settings, key string) []byte {
	if bound, ok := settings.(integrationkinds.ClientBound); ok {
		issuerURL, clientID := bound.AADBinding()

		// NORMALIZED, and it has to be the same normalization the stability
		// guard uses. The guard treats a trailing slash or a differently-cased
		// host as "not a change" and lets the update through without a new
		// secret -- correctly, since discovery trims the slash and the host is
		// case-insensitive. If the AAD were sealed from the raw value, that
		// harmless-looking edit would carry the ciphertext forward under one
		// value and reopen it under another, killing the provider with
		// ErrIntegrationUnreadable on a read nobody connects to the edit.
		//
		// The two must move together. Changing one without the other is how
		// this seam breaks silently, which is why both go through this function.
		return secrets.SecretAADForClient(in.Name(), key, xurl.NormalizeIssuer(issuerURL), clientID)
	}

	return secrets.SecretAAD(in.Name(), key)
}

// sealSecret encrypts one plaintext secret value under dek, bound via AAD to
// its slot (and, for a login provider, to its OAuth client), and returns the
// storable base64(envelope) form. The single seal path — create and update both
// go through here.
func (s *Service) sealSecret(
	in integrationkinds.Integration, settings integrationkinds.Settings, dek []byte, key, value string,
) (string, error) {
	envelope, err := s.cipher.Encrypt(dek, []byte(value), secretAAD(in, settings, key))
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
			// REPLACE: encrypt the new value under the setting's existing DEK,
			// bound to the EFFECTIVE config -- current already carries the patch
			// (Update applies it before calling here), so a login provider's
			// secret is sealed against the identifiers the row will actually
			// hold rather than the ones it is replacing.
			dek, err := dekOnce()
			if err != nil {
				return nil, err
			}
			settings, parseErr := in.Parse(current.Config, nil)
			if parseErr != nil {
				return nil, fmt.Errorf("%w: parse effective config: %w", apperr.ErrValidation, parseErr)
			}
			sealed, err := s.sealSecret(in, settings, dek, key, *intent)
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

// checkAADBindingStable refuses an update that would strand a login provider's
// stored secret.
//
// A ClientBound kind seals its secrets against identifiers taken from the config
// (the issuer URL and the client id). mergeSecrets carries an unchanged
// ciphertext forward WITHOUT re-encrypting it, so an update that changes either
// identifier while leaving a secret unsent would persist a ciphertext bound to
// values the row no longer holds: unopenable, permanently, with the failure
// surfacing later as ErrIntegrationUnreadable on a read nobody connects to this
// edit.
//
// So the rule is an invariant on the AAD inputs rather than a rule about one
// named field: if any of them changes, every secret of the kind must be
// resupplied in the same request. Naming a field instead would leave the hole
// open, because the KEEP branch fires whenever a key is merely absent.
//
// Kinds that are not ClientBound are unaffected — their AAD holds nothing that
// an edit can move.
func (s *Service) checkAADBindingStable(
	in integrationkinds.Integration,
	current *entity.IntegrationSetting,
	cmd *entity.UpdateIntegrationCmd,
	incoming map[string]*string,
) error {
	// A config the caller did not send cannot change the binding.
	if cmd.Config == nil {
		return nil
	}

	storedBinding, ok := aadBindingOf(in, current.Config)
	if !ok {
		return nil // not a ClientBound kind
	}
	incomingBinding, ok := aadBindingOf(in, cmd.Config)
	if !ok {
		return nil
	}

	// Only the AAD inputs matter here: a secret is stranded when the values it
	// was sealed under move, and the wider security fields are not among them.
	storedIssuer, storedClient := storedBinding.aadInputs()
	incomingIssuer, incomingClient := incomingBinding.aadInputs()
	if storedIssuer == incomingIssuer && storedClient == incomingClient {
		return nil
	}

	// The binding moved. Every secret the kind declares must arrive with it.
	for _, key := range in.SecretKeys() {
		if _, hadStored := current.Secrets[key]; !hadStored {
			continue // nothing stored under this key: nothing to strand
		}
		intent, sent := incoming[key]
		if !sent || intent == nil || *intent == "" {
			return fmt.Errorf(
				"%w: changing issuer_url or client_id requires %s to be supplied in the same request, "+
					"because the stored secret is bound to the values being replaced",
				apperr.ErrValidation, key,
			)
		}
	}

	return nil
}

// aadBinding is the comparable form of what a ClientBound kind puts in its AAD,
// plus the fields that change who can sign in without touching the AAD at all.
//
// The two are carried together because one caller needs each: the secret
// stability check compares only the AAD inputs, while the linked-account guard
// compares everything here. Splitting them into two parses would let the two
// answers come from different reads of the same config.
type aadBinding struct {
	issuerURL string
	clientID  string
	// security is opaque: the kind renders its own security-relevant fields
	// into one comparable string, so this layer never has to know their names.
	security string
}

// aadInputs reports just the pair the secret is sealed under.
func (b aadBinding) aadInputs() (issuerURL, clientID string) {
	return b.issuerURL, b.clientID
}

// aadBindingOf parses config and reports the kind's AAD binding, or ok=false if
// the kind is not ClientBound or the config does not parse.
//
// Unparseable config is deliberately not an error here: the write path
// validates it a few lines later with a message about the actual problem, and
// reporting "binding unchanged" for a config that is about to be rejected keeps
// this check from producing a second, more confusing error for the same input.
func aadBindingOf(in integrationkinds.Integration, config json.RawMessage) (aadBinding, bool) {
	settings, err := in.Parse(config, nil)
	if err != nil {
		return aadBinding{}, false
	}
	bound, ok := settings.(integrationkinds.ClientBound)
	if !ok {
		return aadBinding{}, false
	}
	issuerURL, clientID := bound.AADBinding()

	return aadBinding{
		issuerURL: xurl.NormalizeIssuer(issuerURL),
		clientID:  clientID,
		security:  bound.SecurityRelevant(),
	}, true
}
