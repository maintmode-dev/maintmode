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
	// CreatedByUserID is the author (token subject) captured at create time.
	// Nullable: rows that predate authorship carry no author and degrade to the
	// "Unknown user" summary on read.
	CreatedByUserID *uuid.UUID
	UpdatedAt       *time.Time
	// UpdatedByUserID is the last editor (token subject), nil until the resource
	// is first edited.
	UpdatedByUserID *uuid.UUID
}
