package integration

import (
	"encoding/json"
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// WithLoginPresets records the catalog of well-known login providers.
//
// Absent (nil), every login name behaves like `custom`: the operator supplies
// everything. That is the right degenerate behavior for a binary with no
// config -- tests, mostly -- because a preset only ever REMOVES fields from
// what the operator has to type.
func (s *Service) WithLoginPresets(presets config.LoginPresets) *Service {
	s.loginPresets = presets

	return s
}

// applyPreset merges a catalog entry into a create's config.
//
// Three rules, and each exists for a reason worth stating:
//
//   - The preset's values are COPIED INTO the returned config, so the stored
//     row is self-contained. issuer_url is an input to the secret's AAD, so a
//     row that read it from the file at use time would have its secret sealed
//     under a value an operator can edit -- one YAML change would then strand
//     every affected provider behind ErrIntegrationUnreadable at the next
//     sign-in. The cost is that re-pointing a preset provider means deleting
//     and re-creating it; for issuers that have not moved in a decade, that is
//     the right trade.
//
//   - A preset field supplied BY THE CALLER is refused, not ignored and not
//     honored. Honoring it would let an operator create `google` pointed at
//     an IdP they run, which mints whatever subject it likes against the
//     user_identities rows already carrying provider='google' -- account
//     takeover through the exact name the closed set was supposed to make safe.
//     Ignoring it would be quieter but would tell the caller their input took
//     effect when it did not.
//
//   - A name with NO catalog entry is refused rather than treated as "no
//     defaults". Otherwise deleting one line of YAML reopens the path above.
//     `custom` is the exception, and it is an exception because the name itself
//     announces that the operator supplies everything.
func (s *Service) applyPreset(name string, cfg json.RawMessage) (json.RawMessage, error) {
	preset, fields, preseted, err := s.presetAndConfig(name, cfg)
	if err != nil {
		return nil, err
	}
	if !preseted {
		return cfg, nil
	}

	for field, value := range preset.Fields() {
		if _, supplied := fields[field]; supplied {
			return nil, fmt.Errorf("%w: %q is fixed by the %q preset and must not be supplied",
				apperr.ErrValidation, field, name)
		}

		fields[field] = value
	}

	merged, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("apply %q preset: %w", name, err)
	}

	return merged, nil
}

// enforcePreset refuses a config in which a preset-owned field differs from the
// catalog.
//
// Update replaces Config wholesale, so the create-time rule buys nothing
// without this: an operator would create `google` correctly and rewrite its
// issuer one request later, ending up with a provider trusted by every
// user_identities row carrying provider='google' and pointed wherever they
// like.
//
// Shaped as a COMPARISON rather than an injection, unlike applyPreset. On
// create the caller has not sent the preset fields and the values are written
// in; on update the row already carries them, so the question is whether the
// incoming config still agrees. Injecting on update would silently repair a
// tampered field and hide the attempt.
//
// A DROPPED field is refused too. It is the quieter half of the same bug --
// a preset provider left with no issuer at all -- and it reads as an accident
// rather than an attack, which is precisely why it must not pass silently.
func (s *Service) enforcePreset(name string, cfg json.RawMessage) error {
	preset, fields, preseted, err := s.presetAndConfig(name, cfg)
	if err != nil {
		return err
	}
	if !preseted {
		return nil
	}

	for field, want := range preset.Fields() {
		if got, _ := fields[field].(string); got != want {
			return fmt.Errorf("%w: %q is fixed by the %q preset and cannot be changed",
				apperr.ErrValidation, field, name)
		}
	}

	return nil
}

// presetAndConfig is the setup both preset rules need: the catalog entry for a
// name, and the config decoded into a map they can address by key.
//
// Shared because the two rules must agree on WHICH ROWS they apply to. Written
// twice, a later edit to one of the three answers here -- is this name
// preset-backed, does the catalog know it, does the config decode -- would
// leave the other rule judging a different set of rows, and the pair only works
// as a pair: applyPreset fixes the fields at create, enforcePreset keeps them
// fixed at update.
//
// What is NOT shared is what each does with the fields, and that is the whole
// difference between them: one writes the catalog values in, the other compares
// against them. Folding those together with a flag would put "inject or refuse"
// on a boolean, which is exactly the decision that must not be made by accident.
//
// preseted false means the name is not preset-backed (`custom`, or anything
// unregistered) and the caller should do nothing at all.
func (s *Service) presetAndConfig(
	name string, cfg json.RawMessage,
) (preset config.LoginPreset, fields map[string]any, preseted bool, err error) {
	// Whether the catalog speaks for this name is asked of the registered ENTRY,
	// not of the name: `google` and `custom` are the same oidc implementation, so
	// the name alone cannot tell them apart -- the entry carries the key, and an
	// entry without one is not preset-backed. A list of preset-backed names
	// beside the catalog would be a second source of truth to keep in step by
	// hand.
	//
	// An unregistered name answers "not preseted" and falls through to the
	// registry lookup that is about to refuse it anyway.
	in, found := s.registry.lookup(name)
	if !found {
		return config.LoginPreset{}, nil, false, nil
	}

	entry, isPreseted := in.(integrationkinds.Preseted)
	if !isPreseted {
		return config.LoginPreset{}, nil, false, nil
	}

	// The registry says WHICH key; the deployment's catalog says what is under
	// it. Two lookups because they answer different questions -- one is closed
	// in code, the other differs between stands.
	key := entry.PresetKey()
	if key == "" {
		return config.LoginPreset{}, nil, false, nil
	}

	preset, inCatalog := s.loginPresets.For(key)
	if !inCatalog {
		return config.LoginPreset{}, nil, false, fmt.Errorf(
			"%w: %q has no preset in this deployment's catalog", apperr.ErrValidation, key)
	}

	fields = map[string]any{}
	if len(cfg) > 0 {
		if err := json.Unmarshal(cfg, &fields); err != nil {
			return config.LoginPreset{}, nil, false, fmt.Errorf(
				"%w: config: %w", apperr.ErrValidation, err)
		}
	}

	return preset, fields, true, nil
}
