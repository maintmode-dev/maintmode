package authsettingsapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// stubService answers with a fixed set of settings, or a fixed error.
type stubService struct {
	settings []*entity.AuthMethodSetting
	err      error
	lastCmd  *entity.SetAuthMethodEnabledCmd
}

func (s *stubService) List(context.Context) ([]*entity.AuthMethodSetting, error) {
	return s.settings, s.err
}

func (s *stubService) SetEnabled(
	_ context.Context,
	cmd *entity.SetAuthMethodEnabledCmd,
) (*entity.AuthMethodSetting, error) {
	s.lastCmd = cmd
	if s.err != nil {
		return nil, s.err
	}

	return &entity.AuthMethodSetting{Method: cmd.Method, Enabled: cmd.Enabled}, nil
}

func contextWithActor(
	t *testing.T,
	req *http.Request,
	rec *httptest.ResponseRecorder,
	method string,
) *echo.Context {
	t.Helper()

	cfg := echotest.ContextConfig{Request: req, Response: rec}
	if method != "" {
		cfg.PathValues = echo.PathValues{{Name: "method", Value: method}}
	}

	c := cfg.ToContext(t)
	xecho.UserToEchoCtx(c, &entity.User{Email: "admin@example.com"})

	return c
}

func TestList_ReturnsEveryMethod(t *testing.T) {
	t.Parallel()

	svc := &stubService{settings: []*entity.AuthMethodSetting{
		{Method: entity.AuthMethodNameEmailOTP, Enabled: false},
		{Method: entity.AuthMethodNameEmailPassword, Enabled: true},
	}}

	rec := httptest.NewRecorder()
	c := contextWithActor(t, httptest.NewRequest(http.MethodGet, "/api/v1/auth/settings", http.NoBody), rec, "")

	require.NoError(t, New(svc).List(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	methods, ok := got["methods"].([]any)
	require.True(t, ok)
	require.Len(t, methods, 2)

	// Authorship is deliberately NOT in the response: the audit trail already
	// answers who changed what, and a second answer here would be a user lookup
	// on an endpoint that otherwise reads one table.
	first, ok := methods[0].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, first, "updated_by")
	require.NotContains(t, first, "author")
}

// Criterion 12 at the HTTP boundary: an unknown method is a 404, not a 500 and
// not a silently created row.
func TestSetEnabled_UnknownMethodIs404(t *testing.T) {
	t.Parallel()

	svc := &stubService{err: apperr.ErrAuthMethodNotFound}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/settings/sms_otp",
		strings.NewReader(`{"enabled":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := contextWithActor(t, req, rec, "sms_otp")

	require.NoError(t, New(svc).SetEnabled(c))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// The request states the TARGET state rather than asking for a flip, so the
// handler must pass the body's value through untouched -- a handler that
// inverted the current value would turn a retried request into a re-enable.
func TestSetEnabled_PassesTheRequestedStateThrough(t *testing.T) {
	t.Parallel()

	svc := &stubService{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/settings/email_password",
		strings.NewReader(`{"enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := contextWithActor(t, req, rec, "email_password")

	require.NoError(t, New(svc).SetEnabled(c))
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, entity.AuthMethodNameEmailPassword, svc.lastCmd.Method)
	require.False(t, svc.lastCmd.Enabled)
	require.Equal(t, "admin@example.com", svc.lastCmd.Actor.Email)
}
