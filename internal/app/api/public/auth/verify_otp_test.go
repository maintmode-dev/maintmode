package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
	"github.com/ruko1202/maintmode/internal/storages/authcredentials"
	"github.com/ruko1202/maintmode/internal/storages/users"
)

// doVerifyOTP drives the real handler with the given JSON body.
func doVerifyOTP(t *testing.T, impl *Implementation, body string) recordedResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/login/otp/verify", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	c := echotest.ContextConfig{Request: req, Response: rec}.ToContext(t)
	require.NoError(t, impl.VerifyOTP(c))

	return recordedResponse{
		status:       rec.Code,
		body:         rec.Body.String(),
		cacheControl: rec.Header().Get(echo.HeaderCacheControl),
	}
}

// Every failure except the nonce mismatch must be indistinguishable. Each case
// takes a different route — unknown address, no live code, malformed body,
// failed validation — and each would otherwise reach a different arm of the
// shared error mapper.
//
// Asserted pairwise rather than against a literal: what matters is that no two
// differ, and only comparing every pair catches one branch quietly diverging.
func TestVerifyOTP_FailuresAreIndistinguishable(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)
	nonce := strings.Repeat("A", 43) + "="

	got := map[string]recordedResponse{
		"unknown address": doVerifyOTP(t, impl,
			`{"email":"`+uuid.NewString()+`@example.com","code":"123456","session_nonce":"`+nonce+`"}`),
		"no live code": doVerifyOTP(t, impl,
			`{"email":"`+uuid.NewString()+`@example.com","code":"654321","session_nonce":"`+nonce+`"}`),
		// Bind failures, which would otherwise answer 400 through the mapper.
		"wrong json type": doVerifyOTP(t, impl, `{"code":123456}`),
		"truncated json":  doVerifyOTP(t, impl, `{"code":`),
		// Validation failures, which must not be tellable from a wrong code.
		"absent email":    doVerifyOTP(t, impl, `{"code":"123456","session_nonce":"`+nonce+`"}`),
		"malformed email": doVerifyOTP(t, impl, `{"email":"not-an-address","code":"123456","session_nonce":"`+nonce+`"}`),
		"absent code":     doVerifyOTP(t, impl, `{"email":"a@example.com","session_nonce":"`+nonce+`"}`),
		"short code":      doVerifyOTP(t, impl, `{"email":"a@example.com","code":"123","session_nonce":"`+nonce+`"}`),
		"non-digit code":  doVerifyOTP(t, impl, `{"email":"a@example.com","code":"12345a","session_nonce":"`+nonce+`"}`),
		"absent nonce":    doVerifyOTP(t, impl, `{"email":"a@example.com","code":"123456"}`),
		"oversized email": doVerifyOTP(t, impl, `{"email":"`+strings.Repeat("a", 300)+`@example.com","code":"123456","session_nonce":"`+nonce+`"}`),
		"oversized nonce": doVerifyOTP(t, impl, `{"email":"a@example.com","code":"123456","session_nonce":"`+strings.Repeat("n", 500)+`"}`),
		"remember me set": doVerifyOTP(t, impl, `{"email":"a@example.com","code":"123456","session_nonce":"`+nonce+`","remember_me":true}`),

		// The three that traverse the service rather than stopping at bind or
		// validation. They are the ones worth pinning: the cheap rejections all
		// share one code path, while each of these reaches LoginWithOTP, takes a
		// different branch, and carries a different audit reason. If any failure
		// is going to acquire a distinguishing detail, it is one of these.
		"wrong code against a live code": doVerifyOTP(t, impl, seedLiveCode(t, impl)),
		"attempts exhausted":             doVerifyOTP(t, impl, seedBurntCode(t, impl)),
		"expired code":                   doVerifyOTP(t, impl, seedExpiredCode(t, impl)),
	}

	for name, resp := range got {
		require.Equal(t, http.StatusUnauthorized, resp.status, "%s must answer 401", name)

		for otherName, other := range got {
			require.Equal(t, other.status, resp.status, "%s and %s differ in status", name, otherName)
			require.Equal(t, other.body, resp.body, "%s and %s differ in body", name, otherName)
			require.Equal(t, other.cacheControl, resp.cacheControl,
				"%s and %s differ in Cache-Control", name, otherName)
		}
	}

	for name, resp := range got {
		body := strings.ToLower(resp.body)
		require.NotContains(t, body, "expired", "%s leaks the code's state", name)
		require.NotContains(t, body, "attempt", "%s leaks the attempt counter", name)
		require.NotContains(t, body, "nonce", "%s leaks the binding", name)
		require.NotContains(t, body, "not found", "%s leaks whether the account exists", name)
	}
}

// seedRedeemable returns a body that will actually succeed: a real code, read
// back from the sealed delivery task the way the processor would, with its
// matching nonce.
//
// It goes through the real seal and unseal rather than reaching into the store
// for the digest. If issuance ever sealed something verification could not
// match, a test built on a hand-made code would sail past it.
func seedRedeemable(t *testing.T, impl *Implementation) string {
	t.Helper()

	email, cred, nonce := seedAddress(t, impl)

	var raw []byte
	require.NoError(t, db.QueryRowContext(t.Context(),
		"SELECT payload FROM goque_task WHERE type = $1 AND payload->>'credential_id' = $2",
		entity.ProcessorTaskOTPEmailSend, cred.ID.String(),
	).Scan(&raw))

	var task entity.ProcessorTaskPayloadOTPEmail
	require.NoError(t, json.Unmarshal(raw, &task))

	keyring, err := secrets.NewLocalKeyring(cfg.Crypto.ActiveKEKURI, cfg.Crypto.LocalKeys)
	require.NoError(t, err)

	dek, err := keyring.UnwrapDEK(task.DEK, task.KEKURI)
	require.NoError(t, err)

	code, err := secrets.NewAESCipher().Decrypt(dek, task.Code, secrets.OTPCodeAAD(cred.ID.String()))
	require.NoError(t, err)

	return `{"email":"` + email + `","code":"` + string(code) + `","session_nonce":"` + nonce + `"}`
}

// seedWrongNonce returns a body for a real live code carrying somebody else's
// nonce -- the only way to reach the session-mismatch branch.
func seedWrongNonce(t *testing.T, impl *Implementation) string {
	t.Helper()

	email, _, _ := seedAddress(t, impl)

	return verifyBody(email, strings.Repeat("B", 43)+"=")
}

// wrongCode is never the code that was issued.
const wrongCode = "000000"

// seedAddress creates a user and issues a real code for them, returning the
// address, the credential, and the nonce the request handed back.
func seedAddress(t *testing.T, impl *Implementation) (email string, cred *entity.AuthCredential, nonce string) {
	t.Helper()

	email = uuid.NewString() + "@example.com"

	user, err := users.NewStore(db).Create(t.Context(), &entity.User{
		Email: email,
		Name:  "otp verify test user",
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	nonce, err = impl.otpSrv.Request(t.Context(), email)
	require.NoError(t, err)

	cred, err = authcredentials.NewStore(db).GetUnconsumedOTPByUserID(t.Context(), user.ID)
	require.NoError(t, err)

	return email, cred, nonce
}

// seedLiveCode returns a body whose nonce is right and whose code is wrong.
func seedLiveCode(t *testing.T, impl *Implementation) string {
	t.Helper()

	email, _, nonce := seedAddress(t, impl)

	return verifyBody(email, nonce)
}

// seedBurntCode spends a code's whole ceiling, so the next submission is refused
// before any comparison happens.
func seedBurntCode(t *testing.T, impl *Implementation) string {
	t.Helper()

	email, cred, nonce := seedAddress(t, impl)

	store := authcredentials.NewStore(db)
	for range impl.otpSrv.MaxAttempts() {
		claimed, err := store.ClaimOTPAttempt(t.Context(), cred.ID, impl.otpSrv.MaxAttempts())
		require.NoError(t, err)
		require.True(t, claimed)
	}

	return verifyBody(email, nonce)
}

// seedExpiredCode pushes a code's expiry into the past.
func seedExpiredCode(t *testing.T, impl *Implementation) string {
	t.Helper()

	email, cred, nonce := seedAddress(t, impl)

	_, err := db.ExecContext(t.Context(),
		"UPDATE auth_credentials SET expires_at = now() - interval '1 minute' WHERE id = $1", cred.ID)
	require.NoError(t, err)

	return verifyBody(email, nonce)
}

// verifyBody builds a request body carrying a code that is never the one
// issued: these helpers seed the STATE a failure needs (no live code, a spent
// ceiling, an expired row), and the submitted code is incidental to every one.
func verifyBody(email, nonce string) string {
	return `{"email":"` + email + `","code":"` + wrongCode + `","session_nonce":"` + nonce + `"}`
}

// TestVerifyOTP_SessionMismatchIsItsOwnResponse pins the one deliberate
// exception, and pins that it stays narrow: a distinct code and an actionable
// message, with nothing about whose nonce, what was expected, or whether a code
// exists for the address.
func TestVerifyOTP_SessionMismatchIsItsOwnResponse(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	c := echotest.ContextConfig{
		Request:  httptest.NewRequest(http.MethodPost, "/api/v1/login/otp/verify", http.NoBody),
		Response: rec,
	}.ToContext(t)

	// Floor zeroed: this asserts the response body, and waiting out a real floor
	// would only make the test slow.
	impl := initImplWithOTPFloor(t, 0)
	require.NoError(t, impl.otpSessionMismatch(c.Request().Context(), c, time.Now(),
		errors.New("nonce a1b2c3 did not match stored d4e5f6 for user 42")))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get(echo.HeaderCacheControl))
	require.Contains(t, rec.Body.String(), "otp_session_mismatch")
	require.Contains(t, rec.Body.String(), "request a new code")

	body := strings.ToLower(rec.Body.String())
	require.NotContains(t, body, "a1b2c3", "the response must not echo the sent nonce")
	require.NotContains(t, body, "d4e5f6", "the response must not leak the stored nonce")
	require.NotContains(t, body, "user 42", "the response must not identify anyone")
}

// TestOTPRejected_LeaksNothingAboutTheCause is the other half of the collapse:
// the pairwise test proves every path funnels here, this proves what is built
// here is safe to send whatever the cause was.
func TestOTPRejected_LeaksNothingAboutTheCause(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	c := echotest.ContextConfig{
		Request:  httptest.NewRequest(http.MethodPost, "/api/v1/login/otp/verify", http.NoBody),
		Response: rec,
	}.ToContext(t)

	impl := initImplWithOTPFloor(t, 0)
	require.NoError(t, impl.otpRejected(c.Request().Context(), c, time.Now(), "test reason",
		errors.New("code expired for blocked user, attempts exhausted, nonce mismatch")))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get(echo.HeaderCacheControl))

	body := strings.ToLower(rec.Body.String())
	require.NotContains(t, body, "expired")
	require.NotContains(t, body, "blocked")
	require.NotContains(t, body, "exhausted")
	require.NotContains(t, body, "mismatch")
}

// TestVerifyOTP_FloorAppliesToEveryReturnPath is the regression guard for the
// oracle this endpoint actually had.
//
// An address with no account is refused after one indexed SELECT; a real one
// costs a second SELECT, an attempt claim, a hash and a comparison. Uniform
// status and body do not hide that -- a stopwatch separates them, and
// enumeration needs only one request per candidate address, which no rate limit
// bounds. The floor is what closes it, and it only closes it if EVERY return
// path waits.
//
// So the branches below are chosen to leave the handler at different points:
// two before the service is reached at all, one that stops at the user lookup,
// and three that go all the way into verification. A future early return skips
// the floor silently -- the response would still look identical -- so this test
// exists to make that loud.
func TestVerifyOTP_FloorAppliesToEveryReturnPath(t *testing.T) {
	t.Parallel()

	const floor = 250 * time.Millisecond

	impl := initImplWithOTPFloor(t, floor)
	nonce := strings.Repeat("A", 43) + "="

	// The SUCCESS path is floored too, and that is not redundant with the
	// failures. Left unfloored, an attacker holding a victim's code but not
	// their nonce sees otp_session_mismatch at the floor while the correct pair
	// returns immediately -- a timing signal for "the code is right, only the
	// binding is wrong", which is exactly the state a relayed code is in.
	successBody := seedRedeemable(t, impl)

	for name, body := range map[string]string{
		"success": successBody,
		// Never reach the service.
		"truncated json":  `{"code":`,
		"malformed email": `{"email":"not-an-address","code":"123456","session_nonce":"` + nonce + `"}`,
		// Stops at the user lookup -- the fast branch the oracle was made of.
		"unknown address": `{"email":"` + uuid.NewString() + `@example.com","code":"123456","session_nonce":"` + nonce + `"}`,
		// Reach verification proper, each on a different branch.
		"wrong code":         seedLiveCode(t, impl),
		"attempts exhausted": seedBurntCode(t, impl),
		"expired code":       seedExpiredCode(t, impl),
		// The one deliberately distinguishable failure still has to be floored:
		// unfloored it would be the single failure with a telling latency. It
		// needs a REAL live code with the wrong nonce -- a made-up address never
		// reaches the nonce comparison at all.
		"session mismatch": seedWrongNonce(t, impl),
	} {
		start := time.Now()
		resp := doVerifyOTP(t, impl, body)
		elapsed := time.Since(start)

		if name == "success" {
			require.Equal(t, http.StatusOK, resp.status, name)
		} else {
			require.Equal(t, http.StatusUnauthorized, resp.status, name)
		}

		require.GreaterOrEqual(t, elapsed, floor,
			"%s answered in %s, under the %s floor", name, elapsed, floor)
	}
}
