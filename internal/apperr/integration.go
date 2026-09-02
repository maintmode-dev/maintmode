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
	// ErrIntegrationNotConfigured signals that no integration row exists for the
	// kind at all. It WRAPS ErrIntegrationDisabled on purpose: the dispatch path
	// checks errors.Is(err, ErrIntegrationDisabled) for its best-effort drop, and
	// a missing integration must keep dropping exactly like a disabled one. The
	// finer distinction exists for the read model (transport_status
	// not_configured vs disabled).
	ErrIntegrationNotConfigured = fmt.Errorf("%w: not configured", ErrIntegrationDisabled)
	// ErrIntegrationUnreadable signals that an ENABLED integration cannot be
	// resolved locally: its secrets do not decrypt (rolled-back KEK, corrupt
	// envelope, missing DEK row) or its stored settings no longer parse. The
	// read model surfaces it as transport_status "unreadable"; the dispatch path
	// does NOT treat it as a drop — delivery retries toward dead-letter, since
	// the condition is an operational fault, not an admin choice.
	ErrIntegrationUnreadable = errors.New("integration secrets unreadable")
	// ErrUnknownIntegrationKind is returned when a kind has no registered
	// Integration (no parser/validator/transport builder for it).
	ErrUnknownIntegrationKind = fmt.Errorf("%w: unknown integration kind", ErrValidation)
)
