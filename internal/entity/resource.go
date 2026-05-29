package entity

import (
	"time"

	"github.com/google/uuid"
)

// ResourceStatus is the lifecycle state of a resource.
type ResourceStatus string

const (
	ResourceStatusActive   ResourceStatus = "active"
	ResourceStatusArchived ResourceStatus = "archived"
)

type ResourceDetails struct {
	ID          uuid.UUID
	Name        string
	Description string
	ExternalID  *string
	Status      ResourceStatus
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}
