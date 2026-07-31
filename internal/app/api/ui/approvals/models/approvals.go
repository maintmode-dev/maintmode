// Package approvalsmodels holds the wire shapes of the "awaiting my approval"
// screen. Deliberately not named uimodels: the calendar's model package already
// claims that name, and a second one would need an alias at every import site.
package approvalsmodels

import (
	"time"

	"github.com/google/uuid"
)

// UserSummary duplicates the calendar's shape instead of importing it so the two
// contracts can evolve apart.
type UserSummary struct {
	ID uuid.UUID `json:"id" format:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	// DisplayName degrades to "Unknown user" when the author cannot be resolved
	// (blocked, deleted, auth lookup failed). The id is preserved either way.
	DisplayName string `json:"display_name" example:"Ivan Petrov"`
	Email       string `json:"email" example:"ivan.petrov@example.com"`
}

// ApprovalRow is a subset of the calendar event minus status — the listing is
// drafts by definition, so a one-valued column would be noise — plus the two
// timestamps the screen sorts and reasons about.
type ApprovalRow struct {
	ID    uuid.UUID `json:"id" format:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	Title string    `json:"title" example:"Cluster upgrade"`
	Start time.Time `json:"start" example:"2026-08-01T10:00:00Z"`
	// End is not a pointer, matching CalendarEvent: an open-ended period maps to
	// zero-time (0001-01-01T00:00:00Z), which the UI already reads as "no end set".
	End       time.Time    `json:"end" example:"2026-08-01T12:00:00Z"`
	Scope     string       `json:"scope" enums:"global,resource" example:"resource"`
	Impact    string       `json:"impact" example:"degradation"`
	CreatedBy *UserSummary `json:"created_by"`
	CreatedAt time.Time    `json:"created_at" example:"2026-07-20T09:15:00Z"`
	// UpdatedAt is null while the maintenance has not been touched since creation.
	UpdatedAt *time.Time `json:"updated_at" example:"2026-07-28T14:02:00Z"`
}

// ListApprovalsResponse carries one page of the queue. An empty queue serializes
// maintenances as [], never null.
type ListApprovalsResponse struct {
	Maintenances []*ApprovalRow `json:"maintenances"`
	Total        int64          `json:"total" example:"7"`
	Limit        int64          `json:"limit" example:"50"`
	Offset       int64          `json:"offset" example:"0"`
}
