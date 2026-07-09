package notifytransport

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	stubtransport "github.com/ruko1202/maintmode/internal/gateways/notifytransport/stub"
)

// StubResolver short-circuits every delivery to the in-memory stub transport —
// the dev use_stub mode. Which resolver serves deliveries (this or the live
// integration service) is decided once at wiring time in bootstrap, not by a
// per-call branch.
type StubResolver struct {
	stub Transport
}

func NewStubResolver() *StubResolver {
	return &StubResolver{stub: stubtransport.New()}
}

// Get returns the stub for any name, with a warn marker so a stubbed delivery
// is always visible in logs.
func (r *StubResolver) Get(ctx context.Context, name entity.NotifyTransport) (Transport, error) {
	xlog.Warn(ctx, "using stub transport", xfield.String("requested", string(name)))
	return r.stub, nil
}
