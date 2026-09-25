// Package claimer is the invitation side of an invited OAuth dance: it parks
// the invitation behind an opaque handle at /start, resolves it against the
// provider's verified claims before sign-in, and spends it once the user
// exists.
//
// It is its own package, rather than more methods on invitation.Service,
// because the auth service needs it and invitation.Service needs the auth
// service (as its TokenIssuer). The claimer needs no token issuer, so it can be
// built before the auth service and handed to both -- which is what keeps the
// wiring a straight line.
package claimer

import (
	"context"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Store is the invitation persistence the claimer depends on. Defined here
// (consumer side) so tests can substitute a fake.
type Store interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Invitation, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*entity.Invitation, error)
	MarkAccepted(ctx context.Context, id uuid.UUID) (bool, error)
}

// UserService grants an invitation's roles. The seats guard runs inside it, in
// the claim transaction -- see ClaimForUser for why that ordering matters.
type UserService interface {
	AssignRoles(ctx context.Context, cmd *entity.AssignRolesCmd) (*entity.User, error)
}

// DanceHandles parks and redeems the opaque handle an invited OAuth dance
// carries.
//
// Consumer-side: the claimer needs to turn a handle into an invitation id and
// nothing else about the dance. Implemented by the oauthdance store.
type DanceHandles interface {
	PutInvitationHandle(ctx context.Context, handle string, invitationID uuid.UUID) error
	ConsumeInvitationHandle(ctx context.Context, handle string) (*uuid.UUID, error)
}

// Claimer spends invitations: for the invited dance, and for the id_token
// accept path in invitation.Service, which claims through ClaimForUser so the
// transaction exists once.
type Claimer struct {
	txManager    *dbtx.TxManager
	store        Store
	userSrv      UserService
	danceHandles DanceHandles
}

// New builds the claimer.
//
// txManager, store and userSrv must be the ones invitation.Service is built
// from: Accept looks the invitation up through the service's store and spends
// it through this claimer, and the seats guard inside the claim transaction has
// to count against the same rows.
func New(
	txManager *dbtx.TxManager,
	store Store,
	userSrv UserService,
	danceHandles DanceHandles,
) *Claimer {
	return &Claimer{
		txManager:    txManager,
		store:        store,
		userSrv:      userSrv,
		danceHandles: danceHandles,
	}
}
