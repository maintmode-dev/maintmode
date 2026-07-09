// Package publisher provides a shared test double for the AuditPublisher
// capability (Publish(ctx, audit.Action) error) that several services declare
// consumer-side. It replaces the per-package hand-rolled fakes (a capturing spy
// here, a no-op there) with one small implementation.
//
// It is a plain spy rather than a gomock mock on purpose: audit-publishing tests
// wire the double once for a whole suite (TestMain) and later assert on the
// captured actions by type, which a per-test gomock Controller does not fit. For
// call-count/argument expectations on a fresh per-test controller, the generated
// mocks under internal/pkg/generated/mocks remain the right tool.
package publisher

import (
	"context"
	"sync"
	"testing"

	"github.com/ruko1202/maintmode/internal/audit"
)

// Spy records every published audit action so a test can assert the audit trail
// without a real outbox/DB. The zero value is ready to use and behaves as a
// no-op for suites that only need to satisfy the dependency. Safe for concurrent
// Publish (a service may publish from parallel operations).
type Spy struct {
	mu      sync.Mutex
	actions []audit.Action
	t       *testing.T
}

// New returns a Spy bound to t: its captured actions are cleared on t.Cleanup,
// so each test (including parallel ones) gets a fresh, isolated audit trail.
func New(t *testing.T) *Spy {
	t.Helper()

	s := &Spy{t: t}
	t.Cleanup(s.reset)

	return s
}

// Publish records the action and always succeeds — audit publishing is
// best-effort (log-don't-fail) in production, so the double never injects an
// error path a test did not ask for.
func (s *Spy) Publish(_ context.Context, action audit.Action) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions = append(s.actions, action)
	return nil
}

// Actions returns a copy of the captured actions in publish order, so a caller
// can range/assert without racing a concurrent Publish.
func (s *Spy) Actions() []audit.Action {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]audit.Action(nil), s.actions...)
}

func (s *Spy) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions = nil
}
