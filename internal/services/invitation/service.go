// Package invitation orchestrates the user-invitation lifecycle: an admin
// invites an email with pre-assigned roles, the recipient previews the link,
// and accepts it by completing OAuth — which creates the user and issues a
// token pair exactly like a normal login.
//
// Privacy is a first-class constraint: the public preview and accept endpoints
// never reveal invitation contents (email, roles, inviter, message). See
// RUK-94 and the apperr.ErrInvalidInvitation / ErrEmailMismatch comments.
package invitation

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Store is the invitation persistence the service depends on. Defined here
// (consumer side) so tests can substitute a fake.
type Store interface {
	Create(ctx context.Context, inv *entity.Invitation) (*entity.Invitation, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Invitation, error)
	GetForUpdateByID(ctx context.Context, id uuid.UUID) (*entity.Invitation, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*entity.Invitation, error)
	GetActivePendingByEmail(ctx context.Context, email string) (*entity.Invitation, error)
	List(ctx context.Context, cmd *entity.ListInvitationsCmd) ([]*entity.InvitationListItem, error)
	SetRevoked(ctx context.Context, id uuid.UUID, revokedAt time.Time) error
	MarkAccepted(ctx context.Context, id uuid.UUID) (bool, error)
	Resend(ctx context.Context, id uuid.UUID, tokenHash string, expiresAt, sentAt time.Time) (*entity.Invitation, error)
}

// UserService is the subset of user.Service used by the invitation flow.
type UserService interface {
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	GetOrCreateByOAuthInfo(ctx context.Context, provider entity.OAuthProvider, info *entity.OAuthProviderUserInfo, policy entity.UserCreationPolicy) (*entity.User, error)
	AssignRoles(ctx context.Context, cmd *entity.AssignRolesCmd) (*entity.User, error)
}

// TokenIssuer mints a backend token pair for an authenticated user. Implemented
// by auth.Service.IssueTokenPair.
type TokenIssuer interface {
	IssueTokenPair(ctx context.Context, user *entity.User, clientIP string) (*entity.TokenPair, error)
}

// MessageSender enqueues an invitation email through the messaging outbox.
// Implemented by messaging/sender.Service. Defined consumer-side so tests can
// substitute a fake. The enqueue joins the caller's tx (transactional outbox)
// when one is on the context, so it is atomic with the invitation write. The
// task type is explicit so invitation emails land on a queue only the auth binary
// drains (see entity.ProcessorTaskInvitationEmailSend).
type MessageSender interface {
	SendAsync(ctx context.Context, taskType string, trName entity.NotifyTransport, target string, msg entity.NotifyMessage, idempotencyKey string) error
}

// defaultInvitationTTL is used when app.invitation_ttl is not configured.
const defaultInvitationTTL = 7 * 24 * time.Hour

type Service struct {
	txManager      *dbtx.TxManager
	store          Store
	userSrv        UserService
	tokenIssuer    TokenIssuer
	oauthProviders *oauthprovider.Providers
	sender         MessageSender
	ttl            time.Duration
	frontendURL    string
	emailMatch     emailMatcherFunc
}

func NewService(
	cfg *config.AppConfig,
	txManager *dbtx.TxManager,
	store Store,
	userSrv UserService,
	tokenIssuer TokenIssuer,
	oauthProviders *oauthprovider.Providers,
	sender MessageSender,
) *Service {
	invitationTTL := cfg.App.InvitationTTL
	if invitationTTL <= 0 {
		invitationTTL = defaultInvitationTTL
	}

	s := &Service{
		txManager:      txManager,
		store:          store,
		userSrv:        userSrv,
		tokenIssuer:    tokenIssuer,
		oauthProviders: oauthProviders,
		sender:         sender,
		ttl:            invitationTTL,
		frontendURL:    cfg.App.FrontendURL,
		emailMatch:     emailMatchesIgnoreCase,
	}

	if cfg.Environment.IsDev() && cfg.OauthProviders.UseStub {
		s.emailMatch = emailMatchesStub
	}

	return s
}

type emailMatcherFunc func(ctx context.Context, email, invited string) bool

func emailMatchesIgnoreCase(ctx context.Context, email, invited string) bool {
	_, span := xlog.WithOperationSpan(ctx, "service.Invitation.emailMatchesIgnoreCase")
	defer span.End()

	return strings.EqualFold(strings.ToLower(email), strings.ToLower(invited))
}

// always true for stub
func emailMatchesStub(ctx context.Context, _, _ string) bool {
	_, span := xlog.WithOperationSpan(ctx, "service.Invitation.emailMatchesStub")
	defer span.End() //nolint: gocritic

	return true
}
