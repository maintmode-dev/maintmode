package resources

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var db *sqlx.DB

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	os.Exit(m.Run())
}

// recordingAuditPublisher records published actions instead of enqueuing them.
type recordingAuditPublisher struct {
	mu      sync.Mutex
	actions []audit.Action
}

func (p *recordingAuditPublisher) Publish(_ context.Context, action audit.Action) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.actions = append(p.actions, action)

	return nil
}

func (p *recordingAuditPublisher) published() []audit.Action {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]audit.Action(nil), p.actions...)
}
