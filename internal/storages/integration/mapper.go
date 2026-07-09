package integration

import (
	"encoding/json"
	"fmt"

	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

// toDB marshals the entity's config (raw JSON, stored verbatim — opaque to the
// store) and secrets (typed map of already-ciphertext values) into jsonb
// strings. This is the single write-side conversion between the service's
// in-memory shape and the column shape; fromDB is its inverse.
func toDB(s *entity.IntegrationSetting) (*model.IntegrationSettings, error) {
	secretsJSON := "{}"
	if len(s.Secrets) != 0 {
		secretsJSONBytes, err := json.Marshal(s.Secrets)
		if err != nil {
			return nil, fmt.Errorf("integration %q: encode secrets: %w", s.Kind, err)
		}
		secretsJSON = string(secretsJSONBytes)
	}
	return &model.IntegrationSettings{
		ID:              s.ID,
		Kind:            s.Kind,
		Enabled:         s.Enabled,
		Config:          lo.Ternary(len(s.Config) == 0, "{}", string(s.Config)),
		Secrets:         secretsJSON,
		DekID:           s.DEKID,
		CreatedAt:       s.CreatedAt,
		CreatedByUserID: s.CreatedByUserID,
		UpdatedAt:       s.UpdatedAt,
		UpdatedByUserID: s.UpdatedByUserID,
	}, nil
}

// fromDB decodes the stored secrets object once, here at the DB boundary, so
// every downstream consumer works with the typed map. A row whose secrets
// column does not decode is corruption and surfaces as an error, never as an
// empty map.
func fromDB(m *model.IntegrationSettings) (*entity.IntegrationSetting, error) {
	secrets := make(map[string]string)
	// "null" additionally guards rows written before toDB normalized a nil map:
	// unmarshaling the null literal would silently reset the map to nil.
	if m.Secrets != "" && m.Secrets != "{}" && m.Secrets != "null" {
		if err := json.Unmarshal([]byte(m.Secrets), &secrets); err != nil {
			return nil, fmt.Errorf("integration %q: decode stored secrets: %w", m.Kind, err)
		}
	}
	return &entity.IntegrationSetting{
		ID:              m.ID,
		Kind:            m.Kind,
		Enabled:         m.Enabled,
		Config:          lo.Ternary(m.Config == "", json.RawMessage("{}"), json.RawMessage(m.Config)),
		Secrets:         secrets,
		DEKID:           m.DekID,
		CreatedAt:       m.CreatedAt,
		CreatedByUserID: m.CreatedByUserID,
		UpdatedAt:       m.UpdatedAt,
		UpdatedByUserID: m.UpdatedByUserID,
	}, nil
}
