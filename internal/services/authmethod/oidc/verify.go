package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// idTokenClaims is the payload this backend reads off a verified token. The
// registered claims -- iss, aud, exp -- are the verifier's business and are
// already checked by the time these are unmarshalled.
type idTokenClaims struct {
	Email string `json:"email"`
	// EmailVerified is emailVerified rather than bool because issuers disagree
	// about the type: the spec says boolean, and several mint the JSON string
	// "true". encoding/json does not bridge the two -- a plain bool field fails
	// the whole unmarshal with "cannot unmarshal string into Go value of type
	// bool", so one such issuer would have every one of its tokens rejected as
	// malformed.
	EmailVerified emailVerified `json:"email_verified"`
	Name          string        `json:"name"`
	// HostedDomain is Google's `hd`. It stays because allowed_hosted_domains
	// still restricts by it where configured; issuers that do not mint it are
	// unaffected unless that list is set.
	HostedDomain string `json:"hd"`
}

// emailVerified decodes the claim from either a JSON boolean or a JSON string.
// An absent or unrecognized value is false: the claim asserts a check was made,
// so anything we cannot read is "not asserted".
type emailVerified bool

func (e *emailVerified) UnmarshalJSON(raw []byte) error {
	var asBool bool
	if err := json.Unmarshal(raw, &asBool); err == nil {
		*e = emailVerified(asBool)

		return nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err != nil {
		return fmt.Errorf("email_verified is neither a boolean nor a string: %s", raw)
	}

	parsed, err := strconv.ParseBool(asString)
	if err != nil {
		return fmt.Errorf("email_verified string %q is not a boolean", asString)
	}
	*e = emailVerified(parsed)

	return nil
}

func (s *Service) Authenticate(ctx context.Context, idToken string) (*entity.OAuthIDTokenClaims, error) {
	// The instance is a span field rather than part of the name: one name for
	// every provider keeps a query like "how often does verification fail"
	// answerable across all of them, and the field narrows it to one when that
	// is the question.
	ctx, span := xlog.WithOperationSpan(ctx, "service.OAuth.Verify",
		xfield.String("provider", string(s.name)))
	defer span.End()

	// The verifier is built from what discovery named, so an instance whose IdP
	// was unreachable at startup completes itself here.
	verifier, err := s.resolveVerifier(ctx)
	if err != nil {
		xlog.Error(ctx, "oidc provider is not resolved", xfield.Error(err))

		return nil, fmt.Errorf("%w: %w", apperr.ErrAuthUnavailable, err)
	}

	// Signature, issuer, audience and expiry are all checked here. The issuer
	// it compares against is the one the discovery document declared, not the
	// string an operator typed, which is what makes a trailing slash in config
	// harmless.
	token, err := verifier.Verify(ctx, idToken)
	if err != nil {
		xlog.Error(ctx, "verify oidc id token failed", xfield.Error(err))

		// Expiry is the one failure a caller can act on -- it means "sign in
		// again", not "this token is not yours" -- so it keeps its own sentinel.
		//
		// errors.As, not errors.Is: the library reports expiry as a typed error
		// carrying the expiry time, with no sentinel value to compare against,
		// so Is would be false for every one of them. Nothing here reads that
		// time, hence the throwaway target.
		if errors.As(err, new(*gooidc.TokenExpiredError)) {
			return nil, fmt.Errorf("%w: %w", apperr.ErrTokenExpired, err)
		}

		return nil, fmt.Errorf("%w: %w", apperr.ErrInvalidAccessToken, err)
	}

	claims := new(idTokenClaims)
	if err := token.Claims(claims); err != nil {
		xlog.Error(ctx, "decode oidc id token claims failed", xfield.Error(err))

		return nil, fmt.Errorf("%w: %w", apperr.ErrInvalidAccessToken, err)
	}

	if err := validateClaims(ctx, claims, &s.cfg); err != nil {
		xlog.Error(ctx, "invalid oidc id token claims", xfield.Error(err))

		return nil, fmt.Errorf("%w: %w", apperr.ErrInvalidAccessToken, err)
	}

	// Refused separately from a malformed token, and after the signature is
	// checked: the issuer really did sign this, and really did decline to
	// vouch for the address. Email is an identity key wherever it matches an
	// invitation or claims a fresh account, so an unvouched one cannot be let
	// through as if it were checked. There is no alternative path by design.
	if !bool(claims.EmailVerified) {
		xlog.Warn(ctx, "oidc id token reports an unverified email",
			xfield.String("provider", string(s.name)))

		return nil, fmt.Errorf(
			"%w: issuer %s reports email_verified=false", apperr.ErrEmailNotVerified, token.Issuer,
		)
	}

	return &entity.OAuthIDTokenClaims{
		Subject:       token.Subject,
		Email:         claims.Email,
		Name:          claims.Name,
		EmailVerified: bool(claims.EmailVerified),
	}, nil
}

// validateClaims checks the claims the verifier does not: the ones this backend
// reads rather than the ones the protocol defines.
//
// EmailVerified is deliberately absent: it is not a well-formedness rule but a
// policy decision, and collapsing it into "invalid token" is what hid it from
// callers before. Authenticate refuses it separately.
func validateClaims(ctx context.Context, claims *idTokenClaims, cfg *config.JWTVerifierConfig) error {
	return validation.ValidateStructWithContext(ctx, claims,
		validation.Field(&claims.Email, validation.Required),
		validation.Field(&claims.HostedDomain,
			validation.Required.When(len(cfg.AllowedHostedDomains) > 0),
			validation.In(lo.ToAnySlice(cfg.AllowedHostedDomains)...).
				Error(fmt.Sprintf("unexpected hd=%s. expected one of: %v", claims.HostedDomain, cfg.AllowedHostedDomains)),
		),
	)
}
