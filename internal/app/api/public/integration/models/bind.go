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
// the resolved authorship summaries (nil when the id is absent/unresolvable),
// and health is the live state of a login provider -- empty for every delivery
// kind, which omits the field from the response.
func ToAPIIntegration(
	m *entity.MaskedIntegration, author, editor *entity.UserSummary, health string,
) *Integration {
	return &Integration{
		Health:     health,
		ID:         m.ID,
		Kind:       m.Kind,
		Name:       m.Name,
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
// ToAPIIntegrations projects a list, asking healthOf for each row's health.
//
// The list is the admin landing screen, so it is where "I saved it, why does it
// not work" gets answered; returning an empty health here would make an
// operator click into every provider to find the one that did not resolve.
func ToAPIIntegrations(
	items []*entity.MaskedIntegration,
	summaries map[uuid.UUID]*entity.UserSummary,
	healthOf func(*entity.MaskedIntegration) string,
) []*Integration {
	return lo.Map(items, func(m *entity.MaskedIntegration, _ int) *Integration {
		return ToAPIIntegration(m,
			summaries[lo.FromPtr(m.CreatedByUserID)],
			summaries[lo.FromPtr(m.UpdatedByUserID)],
			healthOf(m),
		)
	})
}
