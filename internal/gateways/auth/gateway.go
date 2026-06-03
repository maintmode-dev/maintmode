package authgateway

import (
	"fmt"
	"net/http"

	"github.com/ruko1202/maintmode/internal/config"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
)

const (
	introspectURI = "/api/v1/s2s/introspect"
)

type Gateway struct {
	baseURL    string
	httpClient *http.Client
	// client is the generated, typed auth API client. It shares httpClient as
	// its request doer, so S2S headers and timeouts apply uniformly.
	client authclient.ClientWithResponsesInterface
}

func New(cfg config.ExternalService) (*Gateway, error) {
	httpClient := xhttp.NewClient(
		xhttp.WithS2S(config.GetAppBuildMeta().AppName, cfg.Secret),
		xhttp.WithTimeout(cfg.Timeout),
	)

	client, err := authclient.NewClientWithResponses(
		cfg.GetURL(),
		authclient.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("init auth client: %w", err)
	}

	return &Gateway{
		baseURL:    cfg.GetURL(),
		httpClient: httpClient,
		client:     client,
	}, nil
}
