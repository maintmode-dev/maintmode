package googleoauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MicahParks/jwkset"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

type googleIDTokenClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	HostedDomain  string `json:"hd"`
}

func (s *Service) VerifyToken(ctx context.Context, idToken string) (*entity.OAuthIDTokenClaims, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.OAuth.Google.Verify")
	defer span.End()

	verifyStartedAt := xtime.UTCNow()
	claims := new(googleIDTokenClaims)

	tok, err := jwt.ParseWithClaims(
		idToken,
		claims,
		s.keyfunc.KeyfuncCtx(ctx),
		jwt.WithValidMethods([]string{
			jwt.SigningMethodRS256.Alg(),
			jwt.SigningMethodES256.Alg(),
		}),
		jwt.WithAudience(s.clientID),
		// No jwt.WithIssuer here: Google's verifier config sets the plural
		// jwt_issuers allowlist, never the singular jwt_issuer, so WithIssuer
		// would receive "" — which the library treats as "no issuer check" and
		// silently skips. validateClaims enforces the allowlist below, which is
		// strictly more general (Google mints both "accounts.google.com" and
		// "https://accounts.google.com"). A dead check that looks live is worse
		// than no check.
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(s.cfg.JWTLeeway),
	)
	if err != nil {
		xlog.Error(ctx, "verify google id token failed", xfield.Error(err))

		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %w", apperr.ErrTokenExpired, err)
		}
		if errors.Is(err, jwkset.ErrKeyNotFound) && s.authUnavailable(ctx, verifyStartedAt) {
			return nil, fmt.Errorf("%w: signing key not found", apperr.ErrInvalidAccessToken)
		}

		return nil, fmt.Errorf("%w: %w", apperr.ErrInvalidAccessToken, err)
	}
	if !tok.Valid {
		xlog.Error(ctx, "google id token is invalid")
		return nil, fmt.Errorf("%w: token invalid", apperr.ErrInvalidAccessToken)
	}

	if err := validateClaims(ctx, claims, &s.cfg); err != nil {
		xlog.Error(ctx, "invalid google id token claims", xfield.Error(err))
		return nil, fmt.Errorf("%w: %w", apperr.ErrInvalidAccessToken, err)
	}

	return &entity.OAuthIDTokenClaims{
		Subject: claims.Subject,
		Email:   claims.Email,
		Name:    claims.Name,
	}, nil
}

func (s *Service) authUnavailable(ctx context.Context, verifyStartedAt time.Time) bool {
	keys, err := s.keyfunc.Storage().KeyReadAll(ctx)
	if err != nil {
		xlog.Error(ctx, "read cached jwks keys failed", xfield.Error(err))
		return true
	}

	if len(keys) == 0 {
		return true
	}

	return s.LastRefreshFailedAt(ctx) >= verifyStartedAt.UnixNano()
}

func validateClaims(ctx context.Context, claims *googleIDTokenClaims, cfg *config.JWTVerifierConfig) error {
	return validation.ValidateStructWithContext(ctx, claims,
		validation.Field(&claims.Email, validation.Required),
		validation.Field(&claims.EmailVerified, validation.Required),
		validation.Field(&claims.Subject, validation.Required),
		validation.Field(&claims.Issuer, validation.Required,
			validation.In(lo.ToAnySlice(cfg.JWTIssuers)...).
				Error(fmt.Sprintf("unexpected issuer %s. expected one of: %v", claims.Issuer, cfg.JWTIssuers)),
		),
		validation.Field(&claims.HostedDomain,
			validation.Required.When(len(cfg.AllowedHostedDomains) > 0),
			validation.In(lo.ToAnySlice(cfg.AllowedHostedDomains)...).
				Error(fmt.Sprintf("unexpected hd=%s. expected one of: %v", claims.HostedDomain, cfg.AllowedHostedDomains)),
		),
	)
}
