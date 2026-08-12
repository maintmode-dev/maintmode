package xhttp

import (
	"net/http"
	"time"
)

// WithTimeout overrides the http.Client request timeout. A non-positive value
// is ignored so callers can pass an unset config field without accidentally
// disabling the default NewClient ceiling.
func WithTimeout(timeout time.Duration) ClientOptions {
	return func(c *http.Client) {
		if timeout > 0 {
			c.Timeout = timeout
		}
	}
}

func WithoutRedirect() ClientOptions {
	return WithCustomRedirectFlow(func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	})
}

func WithCustomRedirectFlow(redirectFlow func(_ *http.Request, _ []*http.Request) error) ClientOptions {
	return func(c *http.Client) {
		c.CheckRedirect = redirectFlow
	}
}

func WithCallerBeforeDo(f func(*http.Request)) ClientOptions {
	return func(c *http.Client) {
		tr, ok := c.Transport.(*transport)
		if !ok {
			return
		}

		tr.beforeRoundTrip = append(tr.beforeRoundTrip, f)
	}
}

func WithBearerToken(token string) ClientOptions {
	return WithCallerBeforeDo(withBearerToken(token))
}

// WithBodyLogging enables request and response body dumps at debug level, capped at
// maxLoggedBodyBytes.
//
// It is a LOCAL DEBUGGING tool and is deliberately awkward to reach for: enabling it
// means a code change, a review and a redeploy, so nobody flips it mid-incident.
// Bodies are the one thing redaction cannot make safe — their shape is arbitrary, so
// a secret inside one is unrecognizable — which is why they are off by default and
// why a lint rule forbids this option outside tests.
//
// The type assertion mirrors the other hook options: a client whose Transport was
// replaced silently keeps bodies off, which is the safe direction to fail.
func WithBodyLogging() ClientOptions {
	return func(c *http.Client) {
		tr, ok := c.Transport.(*transport)
		if !ok {
			return
		}

		tr.logBodies = true
	}
}

func WithCallerAfterDo(f func(*http.Response)) ClientOptions {
	return func(c *http.Client) {
		tr, ok := c.Transport.(*transport)
		if !ok {
			return
		}

		//nolint:bodyclose // registers an after-response hook; it neither owns nor reads the body, so there is nothing to close here.
		tr.afterRoundTrip = append(tr.afterRoundTrip, f)
	}
}
