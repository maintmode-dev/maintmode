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
	CreatedAt   time.Time  `json:"created_at" format:"date-time"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" format:"date-time"`
}

type CreateResourceRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ExternalID  *string `json:"external_id,omitempty"`
}

type SearchResourcesRequest struct {
	Name string `query:"name" validate:"required"`
}

type SearchResourcesResponse struct {
	Items []*Resource `json:"resources"`
}
