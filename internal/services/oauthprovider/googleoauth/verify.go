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

func (p *Service) VerifyToken(ctx context.Context, idToken string) (*entity.OAuthIDTokenClaims, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.OAuth.Google.Verify")
	defer span.End()

	verifyStartedAt := xtime.UTCNow()
	claims := new(googleIDTokenClaims)

	tok, err := jwt.ParseWithClaims(
		idToken,
		claims,
		p.jwtVerifier.keyfunc.KeyfuncCtx(ctx),
		jwt.WithValidMethods([]string{
			jwt.SigningMethodRS256.Alg(),
			jwt.SigningMethodES256.Alg(),
		}),
		jwt.WithAudience(p.oauth2Config.ClientID),
		jwt.WithIssuer(p.jwtVerifier.cfg.JWTIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(p.jwtVerifier.cfg.JWTLeeway),
	)
	if err != nil {
		xlog.Error(ctx, "verify google id token failed", xfield.Error(err))

		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %w", apperr.ErrTokenExpired, err)
		}
		if errors.Is(err, jwkset.ErrKeyNotFound) && p.authUnavailable(ctx, verifyStartedAt) {
			return nil, fmt.Errorf("%w: signing key not found", apperr.ErrInvalidAccessToken)
		}

		return nil, fmt.Errorf("%w: %w", apperr.ErrInvalidAccessToken, err)
	}
	if !tok.Valid {
		return nil, fmt.Errorf("%w: token invalid", apperr.ErrInvalidAccessToken)
	}

	if err := validateClaims(ctx, claims, &p.jwtVerifier.cfg); err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrInvalidAccessToken, err)
	}

	return &entity.OAuthIDTokenClaims{
		Subject: claims.Subject,
		Email:   claims.Email,
		Name:    claims.Name,
	}, nil
}

func (p *Service) authUnavailable(ctx context.Context, verifyStartedAt time.Time) bool {
	keys, err := p.jwtVerifier.keyfunc.Storage().KeyReadAll(ctx)
	if err != nil {
		xlog.Error(ctx, "read cached jwks keys failed", xfield.Error(err))
		return true
	}

	if len(keys) == 0 {
		return true
	}

	return p.jwtVerifier.LastRefreshFailedAt(ctx) >= verifyStartedAt.UnixNano()
}

func validateClaims(ctx context.Context, claims *googleIDTokenClaims, cfg *config.JWTVerifierConfig) error {
	return validation.ValidateStructWithContext(ctx, claims,
		validation.Field(&claims.Email, validation.Required),
		validation.Field(&claims.EmailVerified, validation.Required),
		validation.Field(&claims.Subject, validation.Required),
		validation.Field(&claims.Issuer, validation.Required,
			validation.In(cfg.JWTIssuers).Error(fmt.Sprintf("unexpected issuer %s", claims.Issuer)),
		),
		validation.Field(claims.HostedDomain,
			validation.Required.When(len(cfg.AllowedHostedDomains) > 0),
			validation.In(cfg.AllowedHostedDomains).Error(fmt.Sprintf("unexpected hd=%s", claims.HostedDomain)),
		),
	)
}
