package authgateway

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/config"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
)

const extServiceName = "auth"

type Gateway struct {
	// client is the generated, typed auth API client. Its underlying HTTP client
	// carries the S2S headers and timeout, so all calls apply them uniformly.
	client authclient.ClientWithResponsesInterface
}

func New(extServicesCfg config.ExternalServices) (*Gateway, error) {
	cfg, ok := extServicesCfg.Get(extServiceName)
	if !ok {
		return nil, fmt.Errorf("external service 'auth' config is missing")
	}

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
		client: client,
	}, nil
}
