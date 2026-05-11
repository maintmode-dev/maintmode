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

func WithTimeout(timeout time.Duration) ClientOptions {
	return func(c *http.Client) {
		c.Timeout = timeout
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
