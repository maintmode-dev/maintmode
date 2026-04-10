//go:build api

package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/spf13/viper"

	"github.com/ruko1202/maintmode/test/api/client/client"
)

func setupAuthTestClient() *client.Maintmode {
	transport := httptransport.New(
		fmt.Sprintf("%s:%s", viper.GetString(envAPIHost), viper.GetString(envAPIPort)),
		"/auth",
		[]string{"http"},
	)
	return client.New(transport, strfmt.Default)
}

// extractErrorCode extracts HTTP status code from go-openapi errors.
// go-openapi returns typed errors (e.g. *PostAuthS2sV1IntrospectUnauthorized) for known status codes
// and *runtime.APIError for unknown ones.
func extractErrorCode(t *testing.T, err error) int {
	t.Helper()

	var apiErr *runtime.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	if coder, ok := err.(interface{ Code() int }); ok {
		return coder.Code()
	}
	t.Fatalf("cannot extract HTTP status code from error %T: %v", err, err)
	return 0
}
