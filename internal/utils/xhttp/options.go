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
