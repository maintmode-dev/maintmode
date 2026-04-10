//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/auth"
)

func TestAuthAPI_Introspect_WithoutS2SToken(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := auth.NewPostAPIV1S2sIntrospectParams().WithContext(ctx)

	_, err := apiClient.Auth.PostAPIV1S2sIntrospect(params)
	require.Error(t, err)

	code := extractErrorCode(t, err)
	require.Equal(t, http.StatusUnauthorized, code)
}
