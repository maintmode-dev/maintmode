// Package healthcheck provides HTTP handlers for health check endpoints.
// It includes liveness, readiness, and version endpoints for monitoring application health.
package infra

import (
	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/lifecycle"
)

// Implementation provides health check handlers with database connectivity.
type Implementation struct {
	db *sqlx.DB
	// drain is a read-only view of process drain state, owned by main. The
	// readiness handler consults it to report 503 during graceful shutdown so
	// the load balancer ejects this replica — but this layer neither owns nor
	// mutates shutdown state.
	drain lifecycle.DrainChecker
}

// New creates a new health check implementation with the provided database
// connection and a read-only drain checker.
func New(db *sqlx.DB, drain lifecycle.DrainChecker) *Implementation {
	return &Implementation{
		db:    db,
		drain: drain,
	}
}
