package testdbutils

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/users"
)

type UserChanger func(u *entity.User)

func MakeUser(ctx context.Context, t *testing.T, store *users.Store, changers ...UserChanger) *entity.User {
	t.Helper()

	user := &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Name" + t.Name(),
		Roles: entity.DefaultRoles,
	}
	for _, changer := range changers {
		changer(user)
	}

	created, err := store.Create(ctx, user)
	require.NoError(t, err)

	return created
}
