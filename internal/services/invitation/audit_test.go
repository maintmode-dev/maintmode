package invitation

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// recordingAuditPublisher records published actions instead of enqueuing them.
type recordingAuditPublisher struct {
	mu      sync.Mutex
	actions []audit.Action
}

func (p *recordingAuditPublisher) Publish(_ context.Context, action audit.Action) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.actions = append(p.actions, action)

	return nil
}

func (p *recordingAuditPublisher) published() []audit.Action {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]audit.Action(nil), p.actions...)
}

// Issuing an invitation decides who may get in and as what; withdrawing one
// reverses that. Both are recorded, by whom and for which address; the
// idempotent repeat of a revoke changes nothing and records nothing.
func TestInvitation_PublishesAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, mocks := initService(t)

	admin := makeAdmin(ctx, t, svc)
	email := uniqueEmail(t)
	inv, err := svc.Create(ctx, &entity.CreateInvitationCmd{
		Actor: admin,
		Email: email,
		Roles: []entity.Role{entity.RoleEditor},
	})
	require.NoError(t, err)

	require.Equal(t, []audit.Action{
		audit.InvitationCreated{Actor: admin, Invitation: inv},
	}, mocks.audit.published())

	require.NoError(t, svc.Revoke(ctx, &entity.RevokeInvitationCmd{Actor: admin, ID: inv.ID}))
	require.NoError(t, svc.Revoke(ctx, &entity.RevokeInvitationCmd{Actor: admin, ID: inv.ID}))

	published := mocks.audit.published()
	require.Len(t, published, 2, "the second, idempotent revoke must not be recorded")
	revoked, ok := published[1].(audit.InvitationRevoked)
	require.True(t, ok, "got %T", published[1])
	require.Equal(t, admin, revoked.Actor)
	require.Equal(t, inv.ID, revoked.Invitation.ID)
	require.Equal(t, email, revoked.Invitation.Email)
	require.Equal(t, []entity.Role{entity.RoleEditor}, revoked.Invitation.Roles)
}
