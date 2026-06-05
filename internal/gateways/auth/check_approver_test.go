package authgateway

import (
	"context"
	"errors"
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

func newCheckApproverGateway(t *testing.T, handler http.HandlerFunc) *Gateway {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	gw, err := New(config.ExternalService{Protocol: "http", Host: u.Hostname(), Port: port})
	require.NoError(t, err)
	return gw
}

func TestIsEligibleApprover(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("eligible when auth returns the user", func(t *testing.T) {
		t.Parallel()

		var gotQuery url.Values
		gw := newCheckApproverGateway(t, func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"users": [{"id":"` + id.String() + `","email":"r@e.com","display_name":"Rev","roles":["reviewer"]}],
				"total": 1, "limit": 1, "offset": 0
			}`))
		})

		eligible, err := gw.IsEligibleApprover(context.Background(), id)
		require.NoError(t, err)
		require.True(t, eligible)

		// Constrained by id + active=true + the eligible roles.
		require.Equal(t, []string{id.String()}, gotQuery["ids"])
		require.Equal(t, "true", gotQuery.Get("active"))
		require.ElementsMatch(t, []string{"reviewer", "admin"}, gotQuery["roles"])
	})

	t.Run("not eligible when auth returns no user", func(t *testing.T) {
		t.Parallel()

		gw := newCheckApproverGateway(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"users": [], "total": 0, "limit": 1, "offset": 0}`))
		})

		eligible, err := gw.IsEligibleApprover(context.Background(), id)
		require.NoError(t, err)
		require.False(t, eligible)
	})

	t.Run("auth unavailable on non-200", func(t *testing.T) {
		t.Parallel()

		gw := newCheckApproverGateway(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		eligible, err := gw.IsEligibleApprover(context.Background(), id)
		require.Error(t, err)
		require.True(t, errors.Is(err, apperr.ErrAuthUnavailable))
		require.False(t, eligible)
	})
}
