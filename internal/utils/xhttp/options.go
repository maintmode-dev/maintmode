package xhttp

import (
	"net/http"

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
