package entity

import (
	"time"

	"github.com/google/uuid"
)

type ResourceDetails struct {
	ID          uuid.UUID
	Name        string
	Description string
	ExternalID  *string
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}
