package apimodels

import "github.com/google/uuid"

type ResourceType string

const (
	ResourceTypeService  ResourceType = "service"
	ResourceTypeDatabase ResourceType = "database"
	ResourceTypeCluster  ResourceType = "cluster"
)

type Resource struct {
	ID   uuid.UUID    `json:"id" format:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	Type ResourceType `json:"type"`
}
