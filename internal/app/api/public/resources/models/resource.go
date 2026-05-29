package apimodels

import (
	"time"

	"github.com/google/uuid"
)

type Resource struct {
	ID          uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-4466554400000"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ExternalID  *string    `json:"external_id,omitempty"`
	Status      string     `json:"status" example:"active"`
	CreatedAt   time.Time  `json:"created_at" format:"date-time"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" format:"date-time"`
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
