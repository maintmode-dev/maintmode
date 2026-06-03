package authgateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestListActiveUsers(t *testing.T) {
	t.Parallel()

	var gotQuery url.Values
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"users": [{"id":"550e8400-e29b-41d4-a716-446655440000","email":"a@e.com","display_name":"Active","roles":["editor"]}],
			"total": 1, "limit": 50, "offset": 0
		}`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	gw, err := New(config.ExternalService{Protocol: "http", Host: u.Hostname(), Port: port})
	require.NoError(t, err)

	res, err := gw.ListActiveUsers(context.Background(), &entity.ListAssignableUsersQuery{
		Search: "alice",
		Role:   entity.RoleEditor,
		Limit:  50,
	})
	require.NoError(t, err)

	// Hits the S2S users endpoint and always forces active=true so blocked
	// users are never returned.
	require.Equal(t, "/api/v1/s2s/users", gotPath)
	require.Equal(t, "true", gotQuery.Get("active"))
	require.Equal(t, "alice", gotQuery.Get("search"))
	require.Equal(t, "editor", gotQuery.Get("role"))
	require.Equal(t, "50", gotQuery.Get("limit"))

	require.Equal(t, int64(1), res.Total)
	require.Len(t, res.Users, 1)
	require.Equal(t, "Active", res.Users[0].Name)
	require.Equal(t, []entity.Role{entity.RoleEditor}, res.Users[0].Roles)
}
