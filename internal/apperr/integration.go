package apperr

import (
	"errors"
	"fmt"
)

var (
	// ErrIntegrationNotFound is returned when no integration exists for a kind.
	ErrIntegrationNotFound = errors.New("integration not found")
	// ErrIntegrationConflict is returned when creating an integration for a kind
	// that already exists (UNIQUE(kind)).
	ErrIntegrationConflict = errors.New("integration already exists for this kind")
	// ErrIntegrationDisabled signals that an integration exists but is turned off.
	// The notify dispatch path treats it as a best-effort drop, not an error.
	ErrIntegrationDisabled = errors.New("integration is disabled")
	// ErrUnknownIntegrationKind is returned when a kind has no registered
	// Integration (no parser/validator/transport builder for it).
	ErrUnknownIntegrationKind = fmt.Errorf("%w: unknown integration kind", ErrValidation)
)
