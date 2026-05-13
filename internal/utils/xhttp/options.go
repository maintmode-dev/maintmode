package xhttp

import (
	"net/http"
	"time"

	"github.com/ruko1202/maintmode/internal/config"
)

func WithS2S(appName, s2sToken string) ClientOptions {
	return func(c *http.Client) {
		customTr, ok := c.Transport.(*transport)
		if !ok {
			return
		}

		customTr.beforeRoundTrip = append(customTr.beforeRoundTrip, func(req *http.Request) {
			req.Header.Add(config.XS2STokenHeader, s2sToken)
			req.Header.Add(config.XS2STokenAppNameHeader, appName)
		})
	}
}

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
