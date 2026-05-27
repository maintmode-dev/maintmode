package apimodels

import "github.com/google/uuid"

// ResourceRef is a thin reference to a resource by ID, used in maintenance
// request/response payloads. For the full resource shape (name, description,
// timestamps), see apimodels.Resource from resources/models served by
// /api/v1/resources.
type ResourceRef struct {
	ID uuid.UUID `json:"id" format:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
}
