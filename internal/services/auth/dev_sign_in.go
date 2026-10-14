package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// DevSignIn mints a session for a fresh user holding the requested roles. It is
// the dev and test stand-in for a real sign-in: nothing about the caller is
// verified, which is why the route in front of it exists only on stands where
// the stub provider already accepts any token.
//
// The identity is invented here rather than taken from the request, so the
// client cannot steer which account it lands in -- every call is a new user.
func (s *Service) DevSignIn(ctx context.Context, cmd *entity.DevSignInCmd) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.DevSignIn")
	defer span.End()

	// Checked before anything is written: GetOrCreateByAuthInfo rejects an
	// unknown role too, but only after resolveSignInUser has published a
	// login.failed for an identity nobody will ever look up.
	for _, role := range cmd.Roles {
		if !role.Valid(ctx) {
			return nil, fmt.Errorf("%w: %q", apperr.ErrInvalidRole, role)
		}
	}

	id := xuuid.NewString()
	claims := &entity.OAuthIDTokenClaims{
		Subject:       xuuid.NewString(),
		Email:         fmt.Sprintf("%s@mail.com", id),
		Name:          fmt.Sprintf("User Name[%s]", id),
		EmailVerified: true,
	}

	// google, because an identity must reference a registry row and the dev and
	// test stands declare google for exactly this. lo.Uniq, because AssignRoles
	// filters against the roles a user holds, not within one request.
	pair, _, err := s.SignInWithVerifiedClaims(ctx, entity.AuthMethodGoogle, claims, entity.UserCreationPolicy{
		AllowCreate: true,
		GrantRoles:  lo.Uniq(cmd.Roles),
	}, &entity.AuditMetadata{
		IP:        cmd.ClientIP,
		UserAgent: cmd.UserAgent,
	})
	if err != nil {
		return nil, err
	}

	return pair, nil
}
