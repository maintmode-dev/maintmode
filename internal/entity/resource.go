package entity

import (
	"time"

	"github.com/google/uuid"
)

type ResourceType string

const (
	ResourceTypeService  ResourceType = "service"
	ResourceTypeDatabase ResourceType = "database"
	ResourceTypeCluster  ResourceType = "cluster"
)

type Resource struct {
	ID   uuid.UUID
	Type ResourceType
}

type ResourceDetails struct {
	ID          uuid.UUID
	Name        string
	Description string
	ExternalID  *string
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}
