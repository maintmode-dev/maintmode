package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/users"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

func TestIntrospect(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	// Persist the user: Introspect resolves the token subject and rejects tokens
	// whose subject is not an active user (see auth.Introspect block check), so a
	// realistic "active token" test needs a real row, not a synthetic in-memory
	// user.
	// MakeUser defaults to a uuid-unique email/name; keep it (shared DB → emails
	// must be unique per run, see test-data-unique-per-run).
	user := testdbutils.MakeUser(ctx, t, users.NewStore(db))

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		accessToken, err := impl.tokenSrv.IssueAccessToken(ctx, time.Hour, user)
		require.NoError(t, err)

		request := apiauthmodels.IntrospectRequest{AccessToken: accessToken}
		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, request),
		}.ToContextRecorder(t)

		err = impl.Introspect(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apiauthmodels.IntrospectResponse](t, rec.Body)
		require.True(t, resp.Active)
		require.Equal(t, user.ID.String(), resp.Subject)
		require.Equal(t, user.Email, resp.Email)
		require.Equal(t, lo.Map(user.Roles, func(item entity.Role, _ int) string { return string(item) }), resp.Roles)
	})

	t.Run("invalid token", func(t *testing.T) {
		t.Parallel()

		accessToken, err := impl.tokenSrv.IssueAccessToken(ctx, time.Millisecond, user)
		require.NoError(t, err)

		request := apiauthmodels.IntrospectRequest{AccessToken: accessToken}
		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, request),
		}.ToContextRecorder(t)

		err = impl.Introspect(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apiauthmodels.IntrospectResponse](t, rec.Body)
		require.False(t, resp.Active)
	})

	t.Run("empty token", func(t *testing.T) {
		t.Parallel()

		request := apiauthmodels.IntrospectRequest{AccessToken: ""}
		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, request),
		}.ToContextRecorder(t)

		err := impl.Introspect(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		resp := testjsonudils.JSONToAny[apiauthmodels.IntrospectResponse](t, rec.Body)
		require.False(t, resp.Active)
	})
}
