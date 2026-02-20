// Package healthcheck provides HTTP handlers for health check endpoints.
// It includes liveness, readiness, and version endpoints for monitoring application health.
package infra

import (
	"github.com/jmoiron/sqlx"
)

// Implementation provides health check handlers with database connectivity.
type Implementation struct {
	db *sqlx.DB
}

// New creates a new health check implementation with the provided database connection.
func New(db *sqlx.DB) *Implementation {
	return &Implementation{
		db: db,
	}
}
