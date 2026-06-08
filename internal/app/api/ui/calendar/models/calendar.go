package uimodels

import (
	"time"

	"github.com/google/uuid"
)

type CalendarEvent struct {
	ID        uuid.UUID         `json:"id" format:"uuid"`
	Title     string            `json:"title"`
	Start     time.Time         `json:"start"`
	End       time.Time         `json:"end"`
	Scope     string            `json:"scope"`
	Impact    string            `json:"impact"`
	Status    MaintenanceStatus `json:"status"`
	CreatedBy *UserSummary      `json:"created_by"`
}

type CalendarViewRequest struct {
	From        Date                `query:"from" format:"date" example:"2026-01-27"`
	To          Date                `query:"to" format:"date" example:"2026-01-27"`
	Statuses    []MaintenanceStatus `query:"statuses" example:"draft,in_progress"`
	ResourceIDs []uuid.UUID         `query:"resource_ids" format:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	ChannelIDs  []uuid.UUID         `query:"channel_ids" format:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type CalendarViewMeta struct {
	Truncated bool  `json:"truncated"`
	Count     int64 `json:"count"`
}

type CalendarViewResponse struct {
	Events []*CalendarEvent  `json:"events"`
	Meta   *CalendarViewMeta `json:"meta"`
}
