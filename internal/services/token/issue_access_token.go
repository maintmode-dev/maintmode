package token

import (
	"cmp"
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// errNoSession refuses an access token that belongs to no session: without a
// sid nothing can revoke it before it expires.
var errNoSession = errors.New("access token requires a session id")

// IssueAccessToken mints an access token for user inside the session
// (refresh-token family) sessionID. The session is stamped into the token as
// its sid, which is what revoking the session checks it against.
func (s *Service) IssueAccessToken(
	ctx context.Context, accessTokenTTL time.Duration, user *entity.User, sessionID uuid.UUID,
) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.IssueAccessToken")
	defer span.End()

	// A blocked user must not obtain an access token through any path — initial
	// login, refresh-token rotation, and grace-period re-issue all funnel here.
	if user.IsBlocked() {
		xlog.Warn(ctx, "refusing to issue access token for blocked user", xfield.Any("user", user.ID))
		return "", apperr.ErrUserBlocked
	}

	if sessionID == uuid.Nil {
		return "", errNoSession
	}

	now := s.getNowF()

	// OIDC providers (e.g. Google) do not guarantee the `name` claim, and
	// users.name is NOT NULL DEFAULT ''. Without a fallback such a user would get
	// a token with an empty user_name, which RequireAccessToken rejects as
	// invalid (a permanent 401).
	claims := entity.AccessClaims{
		UserName:  cmp.Or(user.Name, user.Email, "unknown"),
		UserEmail: user.Email,
		UserRoles: user.Roles,
		SessionID: sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        xuuid.NewString(), // jti — for the blacklist on logout
			Subject:   user.ID.String(),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = s.kid

	accessToken, err := token.SignedString(s.privateKey)
	if err != nil {
		xlog.Error(ctx, "failed to sign access token", xfield.Error(err))
		return "", err
	}

	return accessToken, nil
}
