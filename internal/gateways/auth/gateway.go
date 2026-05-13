package authgateway

import (
	"net/http"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
)

const (
	introspectURI = "/api/v1/s2s/introspect"
)

type Gateway struct {
	baseURL    string
	httpClient *http.Client
}

func New(cfg config.ExternalService) *Gateway {
	return &Gateway{
		baseURL: cfg.GetURL(),
		httpClient: xhttp.NewClient(
			xhttp.WithS2S(config.GetAppBuildMeta().AppName, cfg.Secret),
			xhttp.WithTimeout(cfg.Timeout),
		),
	}
}
