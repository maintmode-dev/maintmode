package authgateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestIntrospect_Active(t *testing.T) {
	t.Parallel()

	const exp = int64(1900000000)

	var gotPath, gotMethod, gotBody string
	gw := newTestGateway(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"active": true,
			"jti": "jti-123",
			"sub": "user-1",
			"email": "alice@e.com",
			"roles": ["editor", "admin"],
			"exp": 1900000000
		}`))
	})

	claims, err := gw.Introspect(context.Background(), "the-access-token")
	require.NoError(t, err)

	// Hits the S2S introspect endpoint with the token in the JSON body.
	require.Equal(t, "/api/v1/s2s/introspect", gotPath)
	require.Equal(t, http.MethodPost, gotMethod)
	var sent struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal([]byte(gotBody), &sent))
	require.Equal(t, "the-access-token", sent.AccessToken)

	require.Equal(t, "alice@e.com", claims.Email)
	require.ElementsMatch(t, []entity.Role{entity.RoleEditor, entity.RoleAdmin}, claims.Roles)
	require.Equal(t, "jti-123", claims.ID)
	require.Equal(t, "user-1", claims.Subject)
	require.Equal(t, time.Unix(exp, 0), claims.ExpiresAt.Time)
}

func TestIntrospect_Inactive(t *testing.T) {
	t.Parallel()

	gw := newTestGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"active": false}`))
	})

	claims, err := gw.Introspect(context.Background(), "stale-token")
	require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	require.Nil(t, claims)
}

func TestIntrospect_Non200Degrades(t *testing.T) {
	t.Parallel()

	gw := newTestGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	claims, err := gw.Introspect(context.Background(), "the-access-token")
	require.Error(t, err)
	require.ErrorIs(t, err, apperr.ErrAuthUnavailable)
	require.Nil(t, claims)
}
