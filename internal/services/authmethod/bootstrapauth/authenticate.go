package bootstrapauth

import (
	"context"
	"crypto/subtle"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Authenticate checks the supplied password against the configured one and, on
// a match, reports the break-glass admin's identity.
//
// The comparison is crypto/subtle.ConstantTimeCompare and must stay that way:
// this sits behind a permanently-live unauthenticated endpoint, where a
// comparison that returns early on the first differing byte leaks the shared
// prefix length through response timing. This is the only comparison of the
// credential in this package.
//
// The claims are synthetic. Subject is the constant entity.BootstrapSubject —
// there is no upstream provider to issue one — which is what makes a repeat
// login resolve to the same user. Email comes from configuration, never from
// the request: whoever controls the deployment decides who the admin is.
func (s *Service) Authenticate(ctx context.Context, credential string) (*entity.OAuthIDTokenClaims, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.Bootstrap.Authenticate")
	defer span.End()

	// An empty resolved password would otherwise make the empty credential a
	// skeleton key. ResolvePassword never returns one, so this is defense in
	// depth against a future wiring mistake, not a reachable state today.
	if s.password == "" || subtle.ConstantTimeCompare([]byte(credential), []byte(s.password)) != 1 {
		xlog.Warn(ctx, "bootstrap login rejected: credential mismatch")
		return nil, apperr.ErrInvalidCredentials
	}

	return &entity.OAuthIDTokenClaims{
		Subject: entity.BootstrapSubject,
		Email:   s.email,
		Name:    bootstrapUserName,
	}, nil
}
