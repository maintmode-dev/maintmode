package entity

import "github.com/google/uuid"

type ResourceType = string

const (
	ResourceTypeService  ResourceType = "service"
	ResourceTypeDatabase ResourceType = "database"
	ResourceTypeCluster  ResourceType = "cluster"
)

type Resource struct {
	ID   uuid.UUID
	Type ResourceType
}
