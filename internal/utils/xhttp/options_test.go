package xhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// defaultClientTimeout mirrors the ceiling NewClient installs. Duplicated as a
// literal on purpose: if the production default is changed, this test should
// fail and force a deliberate decision, rather than silently tracking whatever
// the new value happens to be.
const defaultClientTimeout = 30 * time.Second

// TestWithTimeoutResolvesToTimeout pins what the option writes to client.Timeout
// for every shape of input, including layered options.
//
// The non-positive rows are the load-bearing half. Deleting `if timeout > 0`
// makes the option assign zero, and http.Client treats a zero Timeout as NO
// timeout at all — a request that hangs forever instead of failing at 30s.
// Nothing else in the suite would notice, since the resulting client still works
// on a fast server.
//
// The layered rows pin application order: the guard must not turn into "first
// positive value wins" (callers expect the later option to override), and it must
// SKIP rather than reset, which the "positive then zero" row is what proves.
func TestWithTimeoutResolvesToTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		timeouts []time.Duration
		want     time.Duration
		why      string
	}{
		{
			name:     "positive value wins over the default",
			timeouts: []time.Duration{2 * time.Second},
			want:     2 * time.Second,
			why:      "catches an option that ignores its argument or writes to the wrong field",
		},
		{
			name:     "zero leaves the default ceiling in place",
			timeouts: []time.Duration{0},
			want:     defaultClientTimeout,
			why:      "a non-positive timeout must never disable the default ceiling",
		},
		{
			name:     "negative leaves the default ceiling in place",
			timeouts: []time.Duration{-1 * time.Second},
			want:     defaultClientTimeout,
			why:      "a non-positive timeout must never disable the default ceiling",
		},
		{
			name:     "the last positive option wins",
			timeouts: []time.Duration{5 * time.Second, 1 * time.Second},
			want:     1 * time.Second,
			why:      "callers layer options and expect the later one to override",
		},
		{
			name:     "a non-positive value does not clear an explicit earlier one",
			timeouts: []time.Duration{5 * time.Second, 0},
			want:     5 * time.Second,
			why:      "the guard must skip rather than reset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := make([]ClientOptions, 0, len(tt.timeouts))
			for _, timeout := range tt.timeouts {
				opts = append(opts, WithTimeout(timeout))
			}

			client := NewClient(opts...)

			assert.Equal(t, tt.want, client.Timeout, tt.why)
			assert.Positive(t, client.Timeout, "the client must never end up with an unlimited timeout")
		})
	}
}

// TestWithTimeoutActuallyExpires proves the field is wired to real behavior.
//
// A test asserting only client.Timeout would pass if the option wrote to a field
// http.Client never consults. Here the server outlives the deadline, so the call
// must fail with a timeout.
func TestWithTimeoutActuallyExpires(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})

	ctx, _ := observedContext(t)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, http.NoBody)
	require.NoError(t, err)

	start := time.Now()
	resp, err := NewClient(WithTimeout(150 * time.Millisecond)).Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}

	require.Error(t, err, "the request must fail once the timeout elapses")
	assert.Less(t, time.Since(start), defaultClientTimeout, "it must fail on the configured timeout, not the default")

	var timeoutErr interface{ Timeout() bool }
	require.ErrorAs(t, err, &timeoutErr)
	assert.True(t, timeoutErr.Timeout(), "the failure must be a timeout, not an unrelated transport error")
}

// TestDefaultClientFollowsRedirects establishes the baseline the redirect
// options are measured against.
//
// Without it, TestWithoutRedirect could be green simply because this httptest
// server never redirected, proving nothing about the option.
func TestDefaultClientFollowsRedirects(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)
	srvURL := redirectServer(t)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL+"/start", http.NoBody)
	require.NoError(t, err)

	resp, err := NewClient().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/end", resp.Request.URL.Path, "by default the client must follow the 302")
}

// TestWithoutRedirect asserts the 302 is surfaced to the caller instead of being
// followed.
//
// Judged by observed behavior — the status code and the Location header — so it
// fails if CheckRedirect is set to something that still follows, or if the option
// returns a plain error (which would surface as err != nil, not a readable 302).
func TestWithoutRedirect(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)
	srvURL := redirectServer(t)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL+"/start", http.NoBody)
	require.NoError(t, err)

	resp, err := NewClient(WithoutRedirect()).Do(req)
	require.NoError(t, err, "ErrUseLastResponse must surface the response, not an error")
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusFound, resp.StatusCode, "the redirect itself must be returned to the caller")
	assert.Equal(t, "/end", resp.Header.Get("Location"))
	assert.Equal(t, "/start", resp.Request.URL.Path, "the client must not have moved on to the target")
}

// TestWithCustomRedirectFlow checks a caller-supplied policy is both invoked
// with the real redirect and honored.
//
// The returned error must reach the caller wrapped in *url.Error, which is what
// distinguishes "the policy was consulted" from "the field was assigned and
// ignored".
func TestWithCustomRedirectFlow(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)
	srvURL := redirectServer(t)

	errRefused := errors.New("redirect refused")

	var (
		mu       sync.Mutex
		calls    int
		gotPath  string
		viaCount int
	)

	client := NewClient(WithCustomRedirectFlow(func(req *http.Request, via []*http.Request) error {
		mu.Lock()
		defer mu.Unlock()

		calls++
		gotPath = req.URL.Path
		viaCount = len(via)

		return errRefused
	}))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL+"/start", http.NoBody)
	require.NoError(t, err)

	// When CheckRedirect returns an error, http.Client returns BOTH the error and
	// the last response, with its body still open — hence the Close below.
	resp, err := client.Do(req)
	require.Error(t, err)
	require.ErrorIs(t, err, errRefused, "the policy's error must reach the caller")
	require.NotNil(t, resp)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusFound, resp.StatusCode, "the caller is handed the unfollowed redirect")

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, 1, calls, "the policy must be consulted exactly once for a single redirect")
	assert.Equal(t, "/end", gotPath, "the policy must observe the redirect TARGET")
	assert.Equal(t, 1, viaCount, "the policy must receive the chain traveled so far")
}

// TestWithCustomRedirectFlowCanAllowRedirect is the mirror case: returning nil
// lets the redirect proceed.
//
// Pairing it with the refusing test above means neither a hardcoded "always
// follow" nor a hardcoded "never follow" implementation can pass both.
func TestWithCustomRedirectFlowCanAllowRedirect(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)
	srvURL := redirectServer(t)

	var called bool
	client := NewClient(WithCustomRedirectFlow(func(_ *http.Request, _ []*http.Request) error {
		called = true
		return nil
	}))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL+"/start", http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.True(t, called, "the policy must be consulted")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/end", resp.Request.URL.Path, "returning nil must let the redirect proceed")
}

// TestWithCallerBeforeDoObservesRealRequest asserts the hook runs against the
// request that is actually sent, not a copy made before mutation.
//
// The hook mutates a header and the server asserts it arrived: that ordering is
// the whole contract, and it fails if the hook is registered but never invoked,
// or invoked after the request has already gone out.
func TestWithCallerBeforeDoObservesRealRequest(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)
	srvURL, seen := echoServer(t)

	var (
		gotMethod string
		gotPath   string
		calls     int
	)

	client := NewClient(WithCallerBeforeDo(func(req *http.Request) {
		calls++
		gotMethod = req.Method
		gotPath = req.URL.Path
		req.Header.Set("X-Before-Hook", "stamped")
	}))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srvURL+"/resource", strings.NewReader(`{}`))
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, 1, calls, "the hook must run exactly once")
	assert.Equal(t, http.MethodPost, gotMethod, "the hook must see the real request")
	assert.Equal(t, "/resource", gotPath)
	assert.Equal(t, "stamped", seen.header.Get("X-Before-Hook"),
		"a mutation made by the hook must reach the wire, so the hook must run before the round trip")
}

// TestWithCallerBeforeDoHooksCompose pins that hooks append rather than replace,
// and run in registration order.
//
// An implementation assigning a single-element slice would drop the first hook
// while still passing the single-hook test above.
func TestWithCallerBeforeDoHooksCompose(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)
	srvURL, seen := echoServer(t)

	var order []string

	client := NewClient(
		WithCallerBeforeDo(func(req *http.Request) {
			order = append(order, "first")
			req.Header.Set("X-First", "1")
		}),
		WithCallerBeforeDo(func(req *http.Request) {
			order = append(order, "second")
			req.Header.Set("X-Second", "2")
		}),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL, http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, []string{"first", "second"}, order, "hooks must run in registration order")
	assert.Equal(t, "1", seen.header.Get("X-First"), "an earlier hook must not be dropped by a later one")
	assert.Equal(t, "2", seen.header.Get("X-Second"))
}

// TestWithCallerAfterDoObservesRealResponse asserts the after-hook receives the
// actual response.
//
// Reading the status and a response header — not just counting invocations —
// means a hook wired up with a zero-value or placeholder response would fail.
func TestWithCallerAfterDoObservesRealResponse(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Marker", "from-server")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	var (
		calls      int
		gotStatus  int
		gotMarker  string
		gotReqPath string
	)

	client := NewClient(WithCallerAfterDo(func(resp *http.Response) {
		calls++
		gotStatus = resp.StatusCode
		gotMarker = resp.Header.Get("X-Marker")
		if resp.Request != nil {
			gotReqPath = resp.Request.URL.Path
		}
	}))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/thing", http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, 1, calls, "the hook must run exactly once")
	assert.Equal(t, http.StatusTeapot, gotStatus, "the hook must observe the real status")
	assert.Equal(t, "from-server", gotMarker, "the hook must observe the real response headers")
	assert.Equal(t, "/thing", gotReqPath)
}

// TestWithCallerAfterDoHooksCompose is the after-side twin of the compose test:
// registration appends, and order is preserved.
func TestWithCallerAfterDoHooksCompose(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)
	srvURL, _ := echoServer(t)

	var order []string

	client := NewClient(
		WithCallerAfterDo(func(_ *http.Response) { order = append(order, "first") }),
		WithCallerAfterDo(func(_ *http.Response) { order = append(order, "second") }),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL, http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, []string{"first", "second"}, order)
}

// TestWithCallerAfterDoIsSkippedOnTransportError documents that the after-hook
// does NOT run when the round trip fails.
//
// RoundTrip returns early on error, so there is no response to hand the hook.
// Pinning it matters both ways: a hook invoked with a nil *http.Response would
// panic inside anything that dereferences it, and callers writing hooks are
// entitled to assume a non-nil argument.
//
// The before-hook is asserted to have run, which proves the request really was
// attempted rather than rejected before the transport was reached.
func TestWithCallerAfterDoIsSkippedOnTransportError(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)

	var beforeCalls, afterCalls int

	client := NewClient(
		WithCallerBeforeDo(func(_ *http.Request) { beforeCalls++ }),
		WithCallerAfterDo(func(_ *http.Response) { afterCalls++ }),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, deadServerURL(t), http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req) //nolint:bodyclose // the round trip fails; resp is nil.
	require.Error(t, err)
	require.Nil(t, resp)

	assert.Equal(t, 1, beforeCalls, "the before-hook must still have run, proving the request was attempted")
	assert.Zero(t, afterCalls, "the after-hook must not run when the round trip failed")
}

// TestHookOptionsSilentlyNoOpOnForeignTransport covers the `if !ok` fallthrough
// in every option that type-asserts *transport.
//
// A client whose Transport was replaced — by a test double, an instrumentation
// wrapper, or a caller building &http.Client{} directly — must survive the
// option unchanged rather than panicking on the failed assertion. Removing the
// `ok` check turns each of these into a nil-map/interface panic at construction
// time, which this catches.
//
// The request that follows is the real assertion: it proves the client is still
// usable, not merely that applying the option did not blow up.
func TestHookOptionsSilentlyNoOpOnForeignTransport(t *testing.T) {
	t.Parallel()

	options := map[string]ClientOptions{
		"WithCallerBeforeDo": WithCallerBeforeDo(func(*http.Request) {}),
		"WithCallerAfterDo":  WithCallerAfterDo(func(*http.Response) {}),
		"WithBearerToken":    WithBearerToken(secret),
		"WithBodyLogging":    WithBodyLogging(),
	}

	for name, opt := range options {
		t.Run(name+" on a replaced transport", func(t *testing.T) {
			t.Parallel()

			ctx, _ := observedContext(t)
			srvURL, _ := echoServer(t)

			client := NewClient()
			client.Transport = http.DefaultTransport

			require.NotPanics(t, func() { opt(client) }, "a foreign transport must not panic the option")
			assert.Same(t, http.DefaultTransport, client.Transport, "the option must leave the transport untouched")

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL, http.NoBody)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err, "the client must remain usable after the no-op option")
			require.NoError(t, resp.Body.Close())
		})

		t.Run(name+" on a bare http.Client", func(t *testing.T) {
			t.Parallel()

			// Transport is nil here, so the assertion fails on the nil interface
			// rather than on a different concrete type — the other shape of the
			// same branch.
			client := &http.Client{}

			require.NotPanics(t, func() { opt(client) })
			assert.Nil(t, client.Transport, "the option must not install a transport of its own")
		})
	}
}

// TestWithBearerTokenDoesNotEngageOnForeignTransport is the behavioral half of
// the branch above for the credential path.
//
// Asserting the header is ABSENT, not merely that nothing panicked: the option
// silently fails to engage, so a caller who replaced the transport gets an
// unauthenticated request. That is the documented behavior and it should fail
// loudly here if it ever changes shape.
func TestWithBearerTokenDoesNotEngageOnForeignTransport(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)
	srvURL, seen := echoServer(t)

	client := NewClient()
	client.Transport = http.DefaultTransport
	WithBearerToken(secret)(client)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL, http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Empty(t, seen.header.Get("Authorization"),
		"the hook cannot engage without our transport, so no credential is attached")
}

// TestWithBodyLoggingFailsClosedOnForeignTransport is the SECURITY-relevant
// direction of the `if !ok` branch.
//
// WithBodyLogging failing to engage is the safe outcome: bodies stay unlogged.
// This asserts that explicitly, so a future refactor that made the option fail
// OPEN — for example by wrapping an unknown transport in ours, or by moving the
// flag onto the client — would surface here as a leak rather than as a quiet
// behavior change. The needle is a secret in the body, which redaction cannot
// help with by design.
func TestWithBodyLoggingFailsClosedOnForeignTransport(t *testing.T) {
	t.Parallel()

	ctx, dump := observedContext(t)
	srvURL, _ := echoServer(t)

	client := NewClient()
	client.Transport = http.DefaultTransport
	WithBodyLogging()(client)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srvURL, strings.NewReader(`{"token":"`+secret+`"}`))
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	out := dump()
	assert.NotContains(t, out, secret, "failing to engage must keep bodies out of the log — the safe direction")
	assert.NotContains(t, out, `"body"`, "no body field may be emitted at all")
}

// TestRedirectOptionsApplyToAnyClient documents the deliberate asymmetry with
// the hook options: the redirect and timeout options write plain http.Client
// fields, so they DO take effect on a client with a foreign transport.
//
// Worth pinning because the two families look interchangeable at a call site.
// If someone "unified" them behind the same *transport assertion, these options
// would start silently no-opping and this test would catch it.
func TestRedirectOptionsApplyToAnyClient(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)
	srvURL := redirectServer(t)

	client := &http.Client{Transport: http.DefaultTransport}
	WithoutRedirect()(client)
	WithTimeout(10 * time.Second)(client)

	assert.Equal(t, 10*time.Second, client.Timeout, "WithTimeout must work on any client")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL+"/start", http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusFound, resp.StatusCode, "WithoutRedirect must work on any client")
}
