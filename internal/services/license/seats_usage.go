package license

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// SeatsUsage reports the seat counts an admin sees before hitting the cap,
// together with the license that caps them. It reuses the very collection the
// heartbeat runs, so the numbers an admin reads and the number the seat guard
// rejects on can never drift apart.
//
// Fail-closed on the counts: a store error surfaces instead of a zeroed report,
// because a zero reads as "plenty of room". The license, in contrast, keeps its
// own fail-open behavior (a read error yields no cap), so an unreadable
// license_cache row shows honest counts with an unknown ceiling rather than an
// error.
//
// The two role reads run in separate statements under READ COMMITTED, so an
// invitation accepted between them is briefly counted in neither half and the
// occupied total dips by one. Accepted deliberately: the window is microseconds
// and this is a polled read whose data is seconds old by contract anyway.
func (s *Service) SeatsUsage(ctx context.Context) (entity.SeatUsage, *entity.License, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.License.SeatsUsage")
	defer span.End()

	usage, err := s.collectSeatUsage(ctx)
	if err != nil {
		xlog.Warn(ctx, "failed to collect seat usage", xfield.Error(err))
		return entity.SeatUsage{}, nil, fmt.Errorf("collect seat usage: %w", err)
	}

	return usage, s.License(ctx), nil
}
