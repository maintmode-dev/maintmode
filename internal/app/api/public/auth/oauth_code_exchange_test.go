package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
)

// exchangeCode drives POST /login/oauth/code/exchange with the given body.
func exchangeCode(t *testing.T, impl *Implementation, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/login/oauth/code/exchange", strings.NewReader(body))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := echotest.ContextConfig{Request: request, Response: rec}.ToContext(t)

	_ = impl.ExchangeOAuthDanceCode(c)

	return rec
}

// danceToCode runs a full dance and returns the one-time code the browser would
// carry to the frontend.
func danceToCode(t *testing.T, impl *Implementation) string {
	t.Helper()

	run := runStart(t, impl)
	rec := callbackRequest(t, impl, url.Values{"code": {"c"}, "state": {run.state}}, run)

	_, q := redirectResult(t, rec)
	code := q.Get("code")
	require.NotEmpty(t, code)

	return code
}

func TestExchangeCodeReturnsThePair(t *testing.T) {
	impl := initDanceImpl(t)
	code := danceToCode(t, impl)

	rec := exchangeCode(t, impl, `{"code":"`+code+`"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var pair apiauthmodels.TokenPairResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pair))
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)

	// The pair is credential material: a cache would put it in a shared proxy.
	assert.Equal(t, "no-store", rec.Header().Get(echo.HeaderCacheControl))
}

// TestExchangeCodeIsSingleUse is the ticket's second named acceptance criterion.
func TestExchangeCodeIsSingleUse(t *testing.T) {
	impl := initDanceImpl(t)
	code := danceToCode(t, impl)

	require.Equal(t, http.StatusOK, exchangeCode(t, impl, `{"code":"`+code+`"}`).Code)

	second := exchangeCode(t, impl, `{"code":"`+code+`"}`)
	assert.Equal(t, http.StatusUnauthorized, second.Code, "a redeemed code must never be redeemable again")
	assert.NotContains(t, second.Body.String(), "access_token")
}

// TestExchangeCodeConcurrentRedemption is what a GET-then-DEL store would fail:
// N callers race for one code and exactly one may leave with a token pair.
// TestExchangeCodeConcurrentRedemption drives the race through the HANDLER.
//
// Written with a release barrier and repeated rounds for the reason the store's
// own concurrency test spells out: goroutines spawned in a loop do not collide
// on their own, and the naive version passes against a store with a genuine
// GET-then-DEL race. Verified by mutation.
func TestExchangeCodeConcurrentRedemption(t *testing.T) {
	impl := initDanceImpl(t)

	const (
		racers = 8
		rounds = 10
	)

	for round := range rounds {
		code := danceToCode(t, impl)

		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			granted int
		)

		start := make(chan struct{})

		wg.Add(racers)
		for range racers {
			go func() {
				defer wg.Done()

				<-start

				if exchangeCode(t, impl, `{"code":"`+code+`"}`).Code == http.StatusOK {
					mu.Lock()
					granted++
					mu.Unlock()
				}
			}()
		}

		close(start)
		wg.Wait()

		require.Equal(t, 1, granted,
			"exactly one racer may redeem a one-time code (round %d)", round)
	}
}

// TestExchangeCodeFailuresAreIndistinguishable pins the uniform answer. An
// attacker probing codes must not learn which of their guesses was structurally
// closer; the audit trail is where the cases stay tellable apart.
func TestExchangeCodeFailuresAreIndistinguishable(t *testing.T) {
	impl := initDanceImpl(t)

	spent := danceToCode(t, impl)
	require.Equal(t, http.StatusOK, exchangeCode(t, impl, `{"code":"`+spent+`"}`).Code)

	bodies := map[string]string{
		"unknown code":  `{"code":"never-issued-at-all"}`,
		"already spent": `{"code":"` + spent + `"}`,
		"empty code":    `{"code":""}`,
		"absent field":  `{}`,
		"malformed":     `{not json`,
	}

	seen := map[string]struct{}{}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			rec := exchangeCode(t, impl, body)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			seen[rec.Body.String()] = struct{}{}
		})
	}

	assert.Len(t, seen, 1, "every failure must answer with one identical body")
}
