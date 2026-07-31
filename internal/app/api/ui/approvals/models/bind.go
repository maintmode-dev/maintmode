package approvalsmodels

import (
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
)

// ToAPIUserSummary maps a resolved domain user summary to the UI shape. Nil-safe:
// a nil summary maps to nil (null).
func ToAPIUserSummary(u *entity.UserSummary) *UserSummary {
	if u == nil {
		return nil
	}

	return &UserSummary{
		ID:          u.ID,
		DisplayName: u.Name,
		Email:       u.Email,
	}
}

// ToAPIApprovalRow maps one pending maintenance to its list row. author is the
// resolved author summary (batch-resolved by the handler); nil when absent.
func ToAPIApprovalRow(maint *calendardto.Maintenance, author *entity.UserSummary) *ApprovalRow {
	return &ApprovalRow{
		ID:        maint.ID,
		Title:     maint.Title,
		Start:     maint.PlannedPeriod.Start,
		End:       lo.FromPtr(maint.PlannedPeriod.End),
		Scope:     string(maint.Scope),
		Impact:    string(maint.Impact),
		CreatedBy: ToAPIUserSummary(author),
		CreatedAt: maint.CreatedAt,
		UpdatedAt: maint.UpdatedAt,
	}
}

// ToAPIApprovalRows maps a page of pending maintenances. lo.Map yields an empty,
// non-nil slice for an empty input, so an empty queue — the most common response
// in production — serializes as [] rather than null.
func ToAPIApprovalRows(
	maints []*calendardto.Maintenance,
	authors map[uuid.UUID]*entity.UserSummary,
) []*ApprovalRow {
	return lo.Map(maints, func(item *calendardto.Maintenance, _ int) *ApprovalRow {
		return ToAPIApprovalRow(item, authors[item.CreatedByUserID])
	})
}

// ApprovalUserIDs collects the author ids of a page so the handler can resolve
// them with a single ResolveMany call.
func ApprovalUserIDs(maints []*calendardto.Maintenance) []uuid.UUID {
	return lo.Map(maints, func(item *calendardto.Maintenance, _ int) uuid.UUID {
		return item.CreatedByUserID
	})
}
