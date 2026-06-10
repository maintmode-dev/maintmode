package auditor

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// GetLogs returns one page of audit log entries plus the total count under
// the full filter and facet counts per category. Facets are computed in the
// actor/date window only (action filter dropped) so the FE category chips
// keep wayfinding numbers while a category is selected.
func (a *Auditor) GetLogs(ctx context.Context, cmd *entity.GetAuditLogsCmd) (*entity.AuditLogsPage, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auditor.GetLogs")
	defer span.End()

	if cmd.Filter == nil {
		cmd.Filter = &entity.AuditFilter{}
	}

	if cmd.Filter.CreatedFrom != nil && cmd.Filter.CreatedTo != nil {
		from, to := lo.FromPtr(cmd.Filter.CreatedFrom), lo.FromPtr(cmd.Filter.CreatedTo)
		if from.After(to) {
			return nil, fmt.Errorf("%w: created_from must be <= created_to", apperr.ErrValidation)
		}
	}

	// Page, total, and facets are separate read-only queries without a shared
	// snapshot: concurrent audit writes may skew them against each other by a
	// few rows. Acceptable drift for an append-only log UI.
	logs, total, err := a.store.GetLogs(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("get audit logs: %w", err)
	}

	actionCounts, err := a.store.CountByAction(ctx, cmd.Filter.WithoutActions())
	if err != nil {
		return nil, fmt.Errorf("count audit logs by action: %w", err)
	}

	return &entity.AuditLogsPage{
		Logs:   logs,
		Total:  total,
		Facets: facetsFromActionCounts(ctx, actionCounts),
	}, nil
}

func facetsFromActionCounts(ctx context.Context, counts map[entity.AuditAction]int64) entity.AuditFacets {
	facets := entity.AuditFacets{}
	for action, count := range counts {
		facets.All += count
		switch category, _ := entity.AuditActionCategory(action); category {
		case entity.AuditCategoryAuth:
			facets.Auth += count
		case entity.AuditCategoryRoles:
			facets.Roles += count
		case entity.AuditCategoryBlock:
			facets.Block += count
		default:
			xlog.Warn(ctx, "unknown action category")
		}
	}
	return facets
}
