package config

import (
	"errors"
	"fmt"
)

// ManagedEntry is what every integration the config file can declare carries,
// whatever its category: who owns it, whether it is on, and its settings. A
// login provider adds its facts beside it (LoginProvider); a notify transport
// is nothing more (NotifyTransportEntry).
//
// ManagedBy says who owns the integration, and is required -- there is no
// default, so no entry falls into a mode by omission:
//
//   - config: the integration is declared HERE. The integration service writes
//     it into the registry at startup, serves it from memory, and refuses every
//     admin write to it. Enabled is required; credentials only when it is true;
//   - ui: an admin creates and runs it in the UI. Settings and Secrets are
//     dropped at load; a leftover Enabled is kept but means nothing -- read
//     ManagedBy before it.
type ManagedEntry struct {
	// ManagedBy is ManagedByConfig or ManagedByUI.
	ManagedBy string `mapstructure:"managed_by"`
	// Enabled is on/off, and nothing else. Required under config, ignored
	// under ui, where the UI switches the integration.
	Enabled *bool `mapstructure:"enabled"`
	// Settings is every other key, flat: client_id, api_url, host, ...
	// After prepare it holds no <secret:KEY> reference.
	Settings map[string]any `mapstructure:",remain"`
	// Secrets is the keys whose value is a <secret:KEY> reference, moved out of
	// Settings at load so the secret resolver -- which rewrites
	// map[string]string values -- resolves them. Never decoded from the file
	// directly.
	Secrets map[string]string `mapstructure:"-"`
}

// The two owners of an integration.
const (
	ManagedByConfig = "config"
	ManagedByUI     = "ui"
)

// prepareEntries checks every entry of one config section and readies it for
// the secret resolver. It runs BEFORE the secrets are applied: telling a
// reference from a literal is only possible while the reference is still
// there, and the resolver only reaches Secrets.
//
// managed reaches the ManagedEntry inside an entry, so one loop serves the
// login section, whose entries carry facts too, and the notify one, whose
// entries are nothing else.
func prepareEntries[E any](section string, entries map[string]E, managed func(*E) *ManagedEntry) error {
	var errs error
	for name, entry := range entries {
		if err := managed(&entry).prepare(); err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s.%s: %w", section, name, err))
			continue
		}

		entries[name] = entry
	}

	return errs
}

// prepare holds the entry to the rules of its mode and moves its <secret:KEY>
// references into Secrets.
//
// A ui entry is emptied: the UI supplies everything, so what the file still
// carries is leftover. Dropping it is what keeps a leftover reference from
// failing startup -- the resolver walks into map[string]any and looks every
// reference up, so one whose key the secrets file no longer carries would stop
// the process over a value nothing reads.
func (e *ManagedEntry) prepare() error {
	if err := e.checkMode(); err != nil {
		return err
	}

	if e.ManagedBy != ManagedByConfig {
		e.Settings, e.Secrets = nil, nil

		return nil
	}

	e.moveSecrets()

	return nil
}

// moveSecrets moves every key whose value is a <secret:KEY> reference from
// Settings into Secrets. What is a secret is what the file marks as one: the
// config cannot know which fields an integration treats as secret -- its kind
// does, and the integration service holds the entry to it.
//
// The move is needed because the secret resolver rewrites map[string]string
// values but cannot reach a value inside map[string]any: a reference left in
// Settings would stay the literal text "<secret:...>".
func (e *ManagedEntry) moveSecrets() {
	for key, value := range e.Settings {
		ref, ok := value.(string)
		if !ok || !secretRegexp.MatchString(ref) {
			continue
		}

		if e.Secrets == nil {
			e.Secrets = map[string]string{}
		}
		e.Secrets[key] = ref
		delete(e.Settings, key)
	}
}

// checkMode holds an entry to the rules of its managed_by. A missing or
// misspelled mode fails startup rather than quietly deciding who owns an
// integration.
func (e *ManagedEntry) checkMode() error {
	switch e.ManagedBy {
	case ManagedByConfig:
		if e.Enabled == nil {
			return errors.New("enabled must be set when managed_by is config")
		}

		return nil

	case ManagedByUI:
		return nil

	default:
		return fmt.Errorf("managed_by must be config or ui, got %q", e.ManagedBy)
	}
}
