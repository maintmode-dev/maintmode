package invitation

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mock_invitation "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/invitation"
	mock_user "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/user"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_oauthprovider "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/oauthprovider"
	"github.com/ruko1202/maintmode/internal/services/auditpublisher"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
	"github.com/ruko1202/maintmode/internal/services/user"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/userinvitations"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

const testInvitationTTL = 7 * 24 * time.Hour

var (
	db  *sqlx.DB
	cfg *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = config.LoadAppConfig()
	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	code := m.Run()
	os.Exit(code)
}

type serviceMocks struct {
	oauthProvider *mock_oauthprovider.MockOAuthProvider
	tokenIssuer   *mock_invitation.MockTokenIssuer
	tokenRevoker  *mock_user.MockTokenRevoker
	sender        *mock_invitation.MockMessageSender

	// sentEmail captures the last message enqueued for email delivery so tests
	// can read the recipient and pull the accept link out of the body.
	sentEmail *sentEmail
}

// sentEmail records what the email transport was asked to deliver.
type sentEmail struct {
	target string
	body   string
}

func initService(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()

	ctrl := gomock.NewController(t)
	txManager := dbtx.NewTxManager(db)

	mocks := &serviceMocks{
		tokenIssuer:   mock_invitation.NewMockTokenIssuer(ctrl),
		tokenRevoker:  mock_user.NewMockTokenRevoker(ctrl),
		oauthProvider: mock_oauthprovider.NewMockOAuthProvider(ctrl),
		sender:        mock_invitation.NewMockMessageSender(ctrl),
		sentEmail:     &sentEmail{},
	}
	mocks.oauthProvider.EXPECT().
		ProviderID().Return(entity.OAuthProviderGoogle).
		AnyTimes()

	// SendAsync captures the enqueued message for assertions and succeeds. The
	// invitation flow enqueues inside its tx (transactional outbox) under the
	// dedicated invitation.email task type.
	mocks.sender.EXPECT().
		SendAsync(gomock.Any(), entity.ProcessorTaskInvitationEmailSend, entity.NotifyTransportEmail, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ entity.NotifyTransport, target string, msg entity.NotifyMessage, _ string) error {
			mocks.sentEmail.target = target
			mocks.sentEmail.body = msg.Body
			return nil
		}).
		AnyTimes()

	return NewService(
		cfg,
		txManager,
		userinvitations.NewStore(db),
		user.NewService(
			txManager,
			users.NewStore(db),
			useridentities.NewStore(db),
			newTestAuditPublisher(t),
			mocks.tokenRevoker,
			false, // allowOpenSignup: the accept flow must authorize creation itself
		),
		mocks.tokenIssuer,
		oauthprovider.NewOAuthProviders(cfg, []oauthprovider.OAuthProvider{mocks.oauthProvider}),
		mocks.sender,
	), mocks
}

// newTestAuditPublisher builds the audit publisher backed by the test DB's goque
// queue. Invitation tests don't assert on audit_log, so events just enqueue
// durably with nothing draining them.
func newTestAuditPublisher(t *testing.T) *auditpublisher.Publisher {
	t.Helper()
	storage, err := goque.NewStorage(db)
	require.NoError(t, err)
	return auditpublisher.New(goque.NewTaskQueueManager(storage))
}

// uniqueEmail returns a per-test-unique email so the shared dev DB stays clean
// across parallel runs.
func uniqueEmail(t *testing.T) string {
	t.Helper()
	return xuuid.NewString() + "@invite-test.com"
}

func mustCreate(ctx context.Context, t *testing.T, s *Service, emailAddr string, roles ...entity.Role) *entity.Invitation {
	t.Helper()
	if len(roles) == 0 {
		roles = []entity.Role{entity.RoleEditor}
	}
	inv, err := s.Create(ctx, &entity.CreateInvitationCmd{
		Actor: makeAdmin(ctx, t, s),
		Email: emailAddr,
		Roles: roles,
	})
	require.NoError(t, err)
	return inv
}

// rawTokenFromLink extracts the raw token query param from a sent invitation
// email body, so tests can drive preview/accept exactly as a recipient would.
// The body is HTML, so the link sits inside an <a href="..."> attribute.
func rawTokenFromLink(t *testing.T, body string) string {
	t.Helper()
	idx := strings.Index(body, "http")
	require.GreaterOrEqual(t, idx, 0, "body has no link: %q", body)
	// Cut at the first delimiter after the URL: the closing quote of the href
	// attribute (HTML), or whitespace.
	link := strings.FieldsFunc(body[idx:], func(r rune) bool {
		return r == '"' || r == '\'' || r == ' ' || r == '\n' || r == '\t' || r == '<'
	})[0]
	u, err := url.Parse(link)
	require.NoError(t, err)
	raw := u.Query().Get("token")
	require.NotEmpty(t, raw)
	return raw
}

func newUUID() uuid.UUID {
	return uuid.MustParse(xuuid.NewString())
}

// makeAdmin creates a real user to act as the inviter (invited_by_id FK).
func makeAdmin(ctx context.Context, t *testing.T, s *Service) *entity.User {
	t.Helper()
	u, err := s.userSrv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "@admin-test.com",
		Name:  "Inviter",
	}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)
	return u
}
