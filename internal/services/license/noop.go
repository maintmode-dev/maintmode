package license

import (
	"context"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Noop is the Enforcement wired when the license feature is not configured
// (self-hosted): the process starts and every consumer holds a working
// provider, but no license is ever known. Callers therefore never branch on
// "is the license configured" — they always call through the interface.
type Noop struct{}

func NewNoop() Noop { return Noop{} }

func (Noop) License(context.Context) *entity.License { return nil }

// EnsureSeatAvailable never enforces a cap on a self-hosted instance: without a
// license every seat grant is unlimited.
func (Noop) EnsureSeatAvailable(context.Context) error { return nil }

// SeatsUsage reports no cap and no counts on a self-hosted instance. Zeros
// rather than a real count: Noop holds no stores, and a nil license renders as
// unlimited, which is the only thing a seat indicator can say here.
func (Noop) SeatsUsage(context.Context) (entity.SeatUsage, *entity.License, error) {
	return entity.SeatUsage{}, nil, nil
}
