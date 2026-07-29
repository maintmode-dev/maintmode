package users

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
)

// TestQueryToListUsersCmdOptsIntoTagSearch pins the admin listing as the one
// search path that matches the telegram/slack tags.
//
// The mirror of this assertion lives in the userpicker service test, which pins
// that the picker leaves the flag off. Both directions need a test: without this
// one, dropping SearchMessengerTags here would silently kill the RUK-217
// scenario (an admin tracing a complaint back to a tag) while every other test
// stayed green — the store-level tag tests set the flag themselves, so they
// cannot notice that production stopped setting it.
func TestQueryToListUsersCmdOptsIntoTagSearch(t *testing.T) {
	t.Parallel()

	c, _ := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet, "/api/v1/users/list?search=%40someone", http.NoBody),
	}.ToContextRecorder(t)

	cmd, err := queryToListUsersCmd(c)
	require.NoError(t, err)

	require.Equal(t, "@someone", cmd.Search)
	require.True(t, cmd.SearchMessengerTags, "the admin list must keep matching messenger tags")
}
