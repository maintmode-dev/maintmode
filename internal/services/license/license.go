package license

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

const licenseCacheKey = cacheKey("licenseCacheKey")

// License returns the current license, read through the short-TTL cache in
// front of the shared license_cache row. Concurrent misses are coalesced into
// a single DB read (see xcache.GetOrLoad). Fail-open: a read error yields a
// non-blocking active license and is never cached, so a DB hiccup never blocks
// the instance. The returned value is shared and must be treated as read-only.
func (s *Service) License(ctx context.Context) *entity.License {
	ctx, span := xlog.WithOperationSpan(ctx, "service.License.License")
	defer span.End()

	license, err := s.cache.GetOrLoad(licenseCacheKey, func() (*entity.License, error) {
		return s.licenseStore.Get(ctx)
	})
	if err != nil {
		return &entity.License{Status: entity.LicenseStatusActive}
	}
	return license
}
