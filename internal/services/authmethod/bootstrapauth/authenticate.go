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

	// The empty check is an INVARIANT, not defense in depth: an unconfigured
	// instance resolves to an empty password, so this is the only thing standing
	// between that state and a skeleton key, and it is reached in normal
	// operation rather than after a wiring mistake. Refusing here rather than
	// declining to register the method is what keeps the attempt on the ordinary
	// failure path -- decoy burned, failure audited, same opaque 401 -- so an
	// instance with no break-glass is indistinguishable from one with a
	// different address configured.
	if s.password == "" || subtle.ConstantTimeCompare([]byte(credential), []byte(s.password)) != 1 {
		xlog.Warn(ctx, "bootstrap login rejected: credential mismatch")
		return nil, apperr.ErrInvalidCredentials
	}

	return &entity.OAuthIDTokenClaims{
		Subject:       entity.BootstrapSubject,
		Email:         s.email,
		Name:          bootstrapUserName,
		EmailVerified: true,
	}, nil
}
