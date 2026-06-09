package apimodels

import (
	"time"

	"github.com/google/uuid"
)

// UserSummary is a privacy-safe view of a user (a resource author or editor)
// exposed in API responses. It mirrors the channel/maintenance UserSummary shape
// so the FE renders authorship the same way everywhere. A nil *UserSummary
// serializes as null (unknown/unset author), so clients must handle the null
// case.
type UserSummary struct {
	ID          uuid.UUID `json:"id" format:"uuid"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

type Resource struct {
	ID          uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-4466554400000"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ExternalID  *string   `json:"external_id,omitempty"`
	Status      string    `json:"status" example:"active"`
	CreatedAt   time.Time `json:"created_at" format:"date-time"`
	// CreatedBy is the resource author resolved from the auth service. Null for
	// legacy rows with no recorded author; when the id is set but unresolvable
	// (auth down or user removed) it degrades to the "Unknown user" label.
	CreatedBy *UserSummary `json:"created_by"`
	// UpdatedAt is null until the resource is first edited.
	UpdatedAt *time.Time `json:"updated_at,omitempty" format:"date-time"`
	// UpdatedBy is the last editor resolved from the auth service, null until the
	// resource is first edited (degrades to "Unknown user" like CreatedBy).
	UpdatedBy *UserSummary `json:"updated_by"`
}

type CreateResourceRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ExternalID  *string `json:"external_id,omitempty"`
}

// UpdateResourceRequest is the body of PATCH /api/v1/resource/{id}. Every field
// is optional: a nil field leaves the value unchanged. external_id may be set
// to an empty string to clear it.
type UpdateResourceRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	ExternalID  *string `json:"external_id,omitempty"`
}

type SearchResourcesRequest struct {
	Name string `query:"name" validate:"required"`
}

type SearchResourcesResponse struct {
	Items []*Resource `json:"resources"`
}

type ListResourcesResponse struct {
	Resources []*Resource `json:"resources"`
	Total     int64       `json:"total" example:"123"`
	Limit     int64       `json:"limit" example:"50"`
	Offset    int64       `json:"offset" example:"0"`
}
