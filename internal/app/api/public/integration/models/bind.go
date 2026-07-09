package apimodels

import (
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

func toAPIUserSummary(u *entity.UserSummary) *UserSummary {
	if u == nil {
		return nil
	}
	return &UserSummary{ID: u.ID, DisplayName: u.Name, Email: u.Email}
}

// ToAPIIntegration maps a masked integration to its API shape. author/editor are
// the resolved authorship summaries (nil when the id is absent/unresolvable).
func ToAPIIntegration(m *entity.MaskedIntegration, author, editor *entity.UserSummary) *Integration {
	return &Integration{
		ID:         m.ID,
		Kind:       m.Kind,
		Enabled:    m.Enabled,
		Config:     m.Config,
		SecretsSet: m.SecretsSet,
		CreatedAt:  m.CreatedAt,
		CreatedBy:  toAPIUserSummary(author),
		UpdatedAt:  m.UpdatedAt,
		UpdatedBy:  toAPIUserSummary(editor),
	}
}

// ToAPIIntegrations maps a list of masked integrations, hydrating each row's
// author/editor from the pre-resolved summary index.
func ToAPIIntegrations(items []*entity.MaskedIntegration, summaries map[uuid.UUID]*entity.UserSummary) []*Integration {
	return lo.Map(items, func(m *entity.MaskedIntegration, _ int) *Integration {
		return ToAPIIntegration(m,
			summaries[lo.FromPtr(m.CreatedByUserID)],
			summaries[lo.FromPtr(m.UpdatedByUserID)],
		)
	})
}
