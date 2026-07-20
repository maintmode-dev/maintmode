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
