package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// This endpoint must not answer the question "does this address have an
// account". The instance is invite-only, so that answer is worth extracting on
// its own, and every branch here would otherwise be distinguishable: an unknown
// address skips the transaction, a malformed one skips the service entirely.
//
// Pairwise equality rather than each against a literal: what matters is that no
// two differ, and only comparing every pair fails when one branch quietly
// diverges.
func TestRequestOTP_OutcomesAreIndistinguishable(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)
	known := makeOTPUser(t, impl)

	got := map[string]recordedOTPResponse{
		"known address":   doRequestOTP(t, impl, `{"email":"`+known+`"}`),
		"blocked account": doRequestOTP(t, impl, `{"email":"`+makeBlockedOTPUser(t, impl)+`"}`),
		"unknown address": doRequestOTP(t, impl, `{"email":"`+uuid.NewString()+`@email.com"}`),
		"absent email":    doRequestOTP(t, impl, `{}`),
		"empty email":     doRequestOTP(t, impl, `{"email":""}`),
		"malformed email": doRequestOTP(t, impl, `{"email":"not-an-address"}`),
		"oversized email": doRequestOTP(t, impl, `{"email":"`+strings.Repeat("a", 300)+`@email.com"}`),
		"wrong json type": doRequestOTP(t, impl, `{"email":123}`),
		"truncated json":  doRequestOTP(t, impl, `{"email":`),
	}

	for name, resp := range got {
		require.Equal(t, http.StatusAccepted, resp.status, "%s must answer 202", name)

		for otherName, other := range got {
			require.Equal(t, other.status, resp.status,
				"%s and %s differ in status", name, otherName)
			require.Equal(t, other.cacheControl, resp.cacheControl,
				"%s and %s differ in Cache-Control", name, otherName)
			require.Equal(t, other.bodyShape, resp.bodyShape,
				"%s and %s differ in body shape", name, otherName)
		}
	}
}

// The nonce is returned on every branch, including those that issued nothing. A
// nonce present only for real accounts would be the oracle the uniform status
// exists to close, moved one field over.
func TestRequestOTP_AlwaysReturnsANonce(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)
	known := makeOTPUser(t, impl)

	for name, body := range map[string]string{
		"known address":   `{"email":"` + known + `"}`,
		"unknown address": `{"email":"` + uuid.NewString() + `@email.com"}`,
		"malformed email": `{"email":"not-an-address"}`,
		"truncated json":  `{"email":`,
	} {
		resp := doRequestOTP(t, impl, body)

		var decoded apiauthmodels.RequestOTPResponse
		require.NoError(t, json.Unmarshal([]byte(resp.body), &decoded), name)
		require.NotEmpty(t, decoded.SessionNonce, "%s returned no nonce", name)
	}
}

// Every return path waits out the floor. Asserted one-sidedly and
// deterministically -- "did this branch return too early" -- rather than by
// comparing two latency distributions, which on a shared test database measures
// the runner rather than the code. The branch that would break this is a future
// early return, and that is exactly what this catches.
func TestRequestOTP_FloorAppliesToEveryReturnPath(t *testing.T) {
	t.Parallel()

	const floor = 250 * time.Millisecond
	impl := initImplWithOTPFloor(t, floor)
	known := makeOTPUser(t, impl)

	for name, body := range map[string]string{
		"known address":   `{"email":"` + known + `"}`,
		"blocked account": `{"email":"` + makeBlockedOTPUser(t, impl) + `"}`,
		"unknown address": `{"email":"` + uuid.NewString() + `@email.com"}`,
		"malformed email": `{"email":"not-an-address"}`,
		"absent email":    `{}`,
		"truncated json":  `{"email":`,
	} {
		start := time.Now()
		resp := doRequestOTP(t, impl, body)
		elapsed := time.Since(start)

		require.Equal(t, http.StatusAccepted, resp.status, name)
		require.GreaterOrEqual(t, elapsed, floor,
			"%s answered in %s, under the %s floor", name, elapsed, floor)
	}
}

// makeOTPUser creates a user this suite can request codes for. Randomized rather
// than derived from the test name: the suite runs with -count 2 against a shared
// database, and a reused address would collide on the second pass.
func makeOTPUser(t *testing.T, impl *Implementation) string {
	t.Helper()

	email := uuid.NewString() + "@email.com"
	_, err := impl.userSrv.GetOrCreateByAuthInfo(
		t.Context(),
		entity.AuthMethodGoogle,
		&entity.OAuthProviderUserInfo{
			ID:    "oauth-" + uuid.NewString(),
			Email: email,
			Name:  "OTP Test User",
		},
		entity.UserCreationPolicy{AllowCreate: true},
	)
	require.NoError(t, err)

	return email
}

// makeBlockedOTPUser creates a user and blocks it. The blocked branch takes a
// third path through the service -- resolved, then refused -- and AC4/AC5 name
// it explicitly: it is the one that leaks account state if it ever diverges.
func makeBlockedOTPUser(t *testing.T, impl *Implementation) string {
	t.Helper()

	email := makeOTPUser(t, impl)

	user, err := impl.userSrv.GetByEmail(t.Context(), email)
	require.NoError(t, err)

	// Actor is another admin, not the user itself: self-blocking is refused by
	// the lockout guard, which is not what this fixture is exercising.
	require.NoError(t, impl.userSrv.BlockUser(t.Context(), &entity.BlockUserCmd{
		Actor:  &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleAdmin}},
		UserID: user.ID,
	}))

	return email
}

type recordedOTPResponse struct {
	status       int
	body         string
	cacheControl string
	// bodyShape is the response with its nonce blanked. The nonce is random per
	// request, so the bytes never match; the shape is what must not vary.
	bodyShape string
}

func doRequestOTP(t *testing.T, impl *Implementation, body string) recordedOTPResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/login/otp/request", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	c := echotest.ContextConfig{Request: req, Response: rec}.ToContext(t)
	require.NoError(t, impl.RequestOTP(c))

	raw := rec.Body.String()

	var decoded apiauthmodels.RequestOTPResponse
	shape := raw
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		decoded.SessionNonce = ""
		normalized, mErr := json.Marshal(decoded)
		require.NoError(t, mErr)
		shape = string(normalized)
	}

	return recordedOTPResponse{
		status:       rec.Code,
		body:         raw,
		cacheControl: rec.Header().Get(echo.HeaderCacheControl),
		bodyShape:    shape,
	}
}
