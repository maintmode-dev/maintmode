package integration

import (
	"encoding/json"
	"slices"
	"strconv"

	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// configChanges diffs two configs field by field for the audit trail: every
// top-level key whose value moved, with its value before and after.
//
// Values are recorded as they are. A config holds no secret -- a secret key is
// refused there (rejectSecretKeysInConfig) -- and every value is already returned
// verbatim by GET to the same admins who read the trail. What the diff adds is
// the one thing GET cannot answer afterwards: that an api_url or a host was
// moved, from where, to where, and by whom.
func configChanges(before, after json.RawMessage) []entity.AuditFieldChange {
	old := configFields(before)
	neu := configFields(after)

	keys := lo.Uniq(append(lo.Keys(old), lo.Keys(neu)...))
	slices.Sort(keys)

	changes := make([]entity.AuditFieldChange, 0, len(keys))
	for _, key := range keys {
		if old[key] == neu[key] {
			continue
		}
		changes = append(changes, entity.AuditFieldChange{Field: key, Old: old[key], New: neu[key]})
	}

	return changes
}

// configFields renders each top-level value for a reader: a string without its
// quotes, anything else as compact JSON. An unreadable config yields nothing to
// compare -- the update itself refuses it with a better message.
func configFields(config json.RawMessage) map[string]string {
	var raw map[string]json.RawMessage
	if len(config) == 0 || json.Unmarshal(config, &raw) != nil {
		return map[string]string{}
	}

	fields := make(map[string]string, len(raw))
	for key, value := range raw {
		var s string
		if json.Unmarshal(value, &s) == nil {
			fields[key] = s
			continue
		}
		fields[key] = compactJSON(value)
	}

	return fields
}

func compactJSON(value json.RawMessage) string {
	var v any
	if json.Unmarshal(value, &v) != nil {
		return string(value)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return string(value)
	}

	return string(out)
}

// secretChanges names each secret this update replaced or cleared, never its
// value: an entry with no Old/New is the trail's "changed" flag, as for a
// maintenance's step list.
func secretChanges(in integrationkinds.Integration, intents map[string]*string) []entity.AuditFieldChange {
	changes := make([]entity.AuditFieldChange, 0, len(intents))
	for _, key := range in.SecretKeys() {
		if _, sent := intents[key]; sent {
			changes = append(changes, entity.AuditFieldChange{Field: "secrets." + key})
		}
	}

	return changes
}

// enabledChange records a flipped enabled flag.
func enabledChange(before, after bool) []entity.AuditFieldChange {
	if before == after {
		return nil
	}

	return []entity.AuditFieldChange{{
		Field: "enabled",
		Old:   strconv.FormatBool(before),
		New:   strconv.FormatBool(after),
	}}
}
