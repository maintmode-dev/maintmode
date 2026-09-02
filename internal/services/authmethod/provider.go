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
// Today every implementation is an upstream OIDC provider, and it deliberately
// does NOT do the authorization-code exchange: the BFF (maintmode-ui, NextAuth)
// owns the OAuth dance with Google and posts us the resulting id_token. The
// backend only verifies that token offline against the provider's JWKS, which
// is why it needs a client_id (the expected audience) but no client_secret.
//
// The credential is typed as a plain string because a password and an emailed
// code will sit behind this same interface. The return type is
// still OAuth-shaped; that asymmetry is temporary and is resolved when the
// first non-OAuth method lands.
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
	if p.useStub {
		xlog.Warn(ctx, "using stub oauth provider")
		methodID = entity.AuthMethodStub
	}

	method, ok := p.methodsStore[methodID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, methodID)
	}

	return method, nil
}
