package router

import (
	"fmt"
	"slices"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Route is the resolved delivery destination for an EventKind.
type Route struct {
	Transport entity.MessengerID
	Target    string
}

// Router resolves an EventKind to []Route from static config.
type Router struct {
	byEvent map[entity.NotifyEventKind][]Route
}

// NewRouter validates each route at construction so misconfig
// (unknown event kind, unknown transport, empty target) surfaces at
// startup, not at first delivery attempt.
func NewRouter(routes []config.MaintNotifyRoute) (*Router, error) {
	byEvent := make(map[entity.NotifyEventKind][]Route)
	for i, raw := range routes {
		kind := entity.NotifyEventKind(raw.Event)
		if !kind.IsValid() {
			return nil, fmt.Errorf("route #%d: unknown event %q", i, raw.Event)
		}

		transport := entity.MessengerID(raw.Transport)
		if !transport.IsValid() {
			return nil, fmt.Errorf("route #%d (event=%s): unknown transport %q", i, raw.Event, raw.Transport)
		}

		if raw.Target == "" {
			return nil, fmt.Errorf("route #%d (event=%s): empty target", i, raw.Event)
		}

		route := Route{Transport: transport, Target: raw.Target}
		if !slices.Contains(byEvent[kind], route) {
			byEvent[kind] = append(byEvent[kind], route)
		}
	}

	return &Router{byEvent: byEvent}, nil
}

func (r *Router) Resolve(kind entity.NotifyEventKind) []Route {
	return r.byEvent[kind]
}
