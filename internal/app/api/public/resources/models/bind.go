package apimodels

import (
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// toAPIUserSummary maps a resolved domain user summary to its API shape.
// Nil-safe: a nil summary maps to nil (serialized as null).
func toAPIUserSummary(u *entity.UserSummary) *UserSummary {
	if u == nil {
		return nil
	}

	return &UserSummary{
		ID:          u.ID,
		DisplayName: u.Name,
		Email:       u.Email,
	}
}

// ToAPIResource maps a domain resource to its API shape. author and editor are
// the authorship summaries resolved from auth (nil when the resource carries no
// corresponding user id, e.g. an unedited resource has no editor).
func ToAPIResource(r *entity.ResourceDetails, author, editor *entity.UserSummary) *Resource {
	return &Resource{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		ExternalID:  r.ExternalID,
		Status:      string(r.Status),
		CreatedAt:   r.CreatedAt,
		CreatedBy:   toAPIUserSummary(author),
		UpdatedAt:   r.UpdatedAt,
		UpdatedBy:   toAPIUserSummary(editor),
	}
}

// ToAPIResources maps a page of resources to their API shape, looking up each
// resource's author/editor summary in the pre-resolved index (keyed by user id).
func ToAPIResources(resources []*entity.ResourceDetails, summaries map[uuid.UUID]*entity.UserSummary) []*Resource {
	return lo.Map(resources, func(r *entity.ResourceDetails, _ int) *Resource {
		return ToAPIResource(r,
			lookupSummary(summaries, r.CreatedByUserID),
			lookupSummary(summaries, r.UpdatedByUserID),
		)
	})
}

// lookupSummary resolves a (possibly nil) user id against the summary index.
func lookupSummary(summaries map[uuid.UUID]*entity.UserSummary, id *uuid.UUID) *entity.UserSummary {
	if id == nil {
		return nil
	}
	return summaries[*id]
}

// ResourceUserIDs collects every non-nil author/editor id across the page, for a
// single batched ResolveMany call.
func ResourceUserIDs(resources []*entity.ResourceDetails) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(resources)*2)
	for _, r := range resources {
		if r.CreatedByUserID != nil {
			ids = append(ids, *r.CreatedByUserID)
		}
		if r.UpdatedByUserID != nil {
			ids = append(ids, *r.UpdatedByUserID)
		}
	}
	return ids
}
