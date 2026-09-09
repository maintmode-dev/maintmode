package authmethod

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/authmethod/stuboauth"
)

// AuthMethod verifies an identity asserted by a credential.
//
// Verification through this interface is offline and credential-free: an OIDC
// implementation checks an ID token against its provider's JWKS, so it needs a
// client_id as the expected audience but no client_secret. The confidential
// half -- the authorization-code exchange, which does hold the secret -- lives
// in gateways/oidc. Keeping the split means both sign-in paths, the BFF one and
// the backend dance, arrive at the same checks instead of trusting
// asymmetrically.
//
// The credential is typed as a plain string because its meaning belongs to the
// implementation: an ID token for OIDC, a password for break-glass, an email
// address for the dev stub. The return type is still OAuth-shaped, and that
// asymmetry resolves when a method lands that has no upstream at all.
type AuthMethod interface {
	// MethodID returns the method's ID.
	MethodID() entity.AuthMethod
	// Authenticate validates a credential and returns the trusted subset of
	// claims used to identify a user.
	Authenticate(ctx context.Context, credential string) (*entity.OAuthIDTokenClaims, error)
}

type Methods struct {
	useStub      bool
	methodsStore map[entity.AuthMethod]AuthMethod
	// danceProviders is the subset that can run the backend dance -- those with
	// a client secret and a callback URL configured.
	danceProviders map[entity.AuthMethod]struct{}
}

func NewAuthMethods(
	cfg *config.AppConfig,
	methods []AuthMethod,
) *Methods {
	methodsMap := lo.SliceToMap(methods, func(item AuthMethod) (entity.AuthMethod, AuthMethod) {
		return item.MethodID(), item
	})

	// The stub accepts any token and mints an identity, so it must not merely be
	// deprioritized outside dev — it must not exist. Registering it
	// unconditionally made Get("stub") resolve in prod regardless of useStub,
	// leaving the invitation email check as the only thing standing between a
	// forged token and a session.
	isDev := cfg.Environment.IsDev()
	if isDev {
		methodsMap[entity.AuthMethodStub] = stuboauth.NewService()
	}

	// useStub keeps its own meaning — "substitute the stub for ANY method" —
	// and is a separate decision from whether the stub exists at all. It stays
	// derived from the same isDev so the two can never disagree: a true useStub
	// with no stub registered would make Get fail for every method.
	return &Methods{
		useStub:      isDev && cfg.OauthProviders.UseStub,
		methodsStore: methodsMap,
	}
}

func (p *Methods) Get(ctx context.Context, methodID entity.AuthMethod) (AuthMethod, error) {
	// The break-glass method is exempt from the stub substitution. The stub
	// accepts any credential and reports Subject "stub", so substituting it here
	// would not merely bypass the password check — it would resolve a DIFFERENT
	// user than the bootstrap identity, silently breaking "a repeat login
	// resolves the same user" while appearing to work. Both the dev and test
	// stands ship use_stub: true, so this is the common case there, not a corner.
	if p.useStub && methodID != entity.AuthMethodBootstrap {
		xlog.Warn(ctx, "using stub oauth provider")
		methodID = entity.AuthMethodStub
	}

	method, ok := p.methodsStore[methodID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, methodID)
	}

	return method, nil
}

// Parse resolves a provider name a client named in a request.
//
// The vocabulary is what is REGISTERED, so an instance added to configuration
// is accepted here without a code change -- which is the point of configuring
// providers rather than compiling them in.
//
// Two names are refused whatever the registry holds, and the reasons predate
// any of this:
//
//   - stub verifies nothing. It exists only on a dev stand, and a client naming
//     it is a client asking to skip verification.
//   - bootstrap carries privileges no other method has: an identity resolved by
//     configured email, and an admin grant that skips the seats cap. Those are
//     safe only on the endpoint that gates them behind the break-glass secret.
//     Letting a client NAME the method elsewhere would carry them onto a flow
//     that never intended them.
//
// Neither can be reopened by configuration: config.ValidateInstanceKey reserves
// both names, so no instance can be registered under them.
//
// Matching is exact -- no case folding, which would let "STUB" smuggle the stub
// past this gate.
func (p *Methods) Parse(name string) (entity.AuthMethod, bool) {
	method := entity.AuthMethod(name)
	if method == entity.AuthMethodStub || method == entity.AuthMethodBootstrap {
		return "", false
	}

	if _, ok := p.methodsStore[method]; !ok {
		return "", false
	}

	return method, true
}

// DanceProvider resolves a {provider} path segment to the method that serves it,
// reporting whether the backend-driven dance supports it.
//
// Narrower than Parse: a provider is only danceable if a gateway was built for
// it, which needs the confidential-client credentials. The check happens before
// any other work in /start because the segment shares a path space with the
// static /login/oauth/code/exchange route, so an unvalidated parameter is how a
// request for one route ends up served by another.
func (p *Methods) DanceProvider(segment string) (entity.AuthMethod, bool) {
	method, ok := p.Parse(segment)
	if !ok {
		return "", false
	}

	if _, danceable := p.danceProviders[method]; !danceable {
		return "", false
	}

	return method, true
}

// WithDanceProviders records which registered providers can run the dance.
func (p *Methods) WithDanceProviders(names []string) *Methods {
	p.danceProviders = make(map[entity.AuthMethod]struct{}, len(names))
	for _, name := range names {
		p.danceProviders[entity.AuthMethod(name)] = struct{}{}
	}

	return p
}
