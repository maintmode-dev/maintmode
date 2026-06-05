package authgateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
)

func newTestGateway(t *testing.T, h http.HandlerFunc) *Gateway {
	t.Helper()

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	gw, err := New(config.ExternalServices{extServiceName: {Protocol: "http", Host: u.Hostname(), Port: port}})
	require.NoError(t, err)
	return gw
}

func TestGetUsersByIDs_Empty(t *testing.T) {
	t.Parallel()

	called := false
	gw := newTestGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	got, err := gw.GetUsersByIDs(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, got)
	require.False(t, called, "empty input must short-circuit without calling auth")
}

func TestGetUsersByIDs_Resolved(t *testing.T) {
	t.Parallel()

	id1 := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	id2 := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")

	var gotQuery url.Values
	var gotPath string
	gw := newTestGateway(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"users": [
				{"id":"550e8400-e29b-41d4-a716-446655440000","email":"a@e.com","display_name":"Alice","roles":["editor"]},
				{"id":"550e8400-e29b-41d4-a716-446655440001","email":"b@e.com","display_name":"Bob","roles":["admin"]}
			],
			"total": 2, "limit": 2, "offset": 0
		}`))
	})

	got, err := gw.GetUsersByIDs(context.Background(), []uuid.UUID{id1, id2})
	require.NoError(t, err)

	// Hits the S2S users endpoint with the ids filter and, unlike the active-
	// users path, does NOT force active=true (blocked authors must still resolve).
	require.Equal(t, "/api/v1/s2s/users", gotPath)
	require.ElementsMatch(t, []string{id1.String(), id2.String()}, gotQuery["ids"])
	require.Empty(t, gotQuery.Get("active"))

	require.Len(t, got, 2)
	require.Equal(t, "Alice", got[id1].Name)
	require.Equal(t, "b@e.com", got[id2].Email)
}

func TestGetUsersByIDs_Non200Degrades(t *testing.T) {
	t.Parallel()

	gw := newTestGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	got, err := gw.GetUsersByIDs(context.Background(), []uuid.UUID{uuid.New()})
	require.Error(t, err)
	require.ErrorIs(t, err, apperr.ErrAuthUnavailable)
	require.Nil(t, got)
}
