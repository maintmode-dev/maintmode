//go:build api

package api

import (
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
)

// setupAuthTestClient builds an unauthenticated auth-service client. Auth
// endpoints under test (login, refresh, callback, jwks, exchange) are reached
// without a Bearer token; authorized flows use setupAuthTestClientWithRoles
// from main_test.go.
func setupAuthTestClient() *authclient.ClientWithResponses {
	return newAuthTestClient("")
}
