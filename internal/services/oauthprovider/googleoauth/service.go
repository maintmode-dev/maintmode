package googleoauth

import (
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/ruko1202/maintmode/internal/utils/xhttp"

	"github.com/ruko1202/maintmode/internal/config"
)

// Service implements repository.GoogleOAuthProvider using golang.org/x/oauth2.
type Service struct {
	config            *oauth2.Config
	googleUserInfoURL string
	httpClient        *http.Client
}

// NewService creates a GoogleOAuth provider.
func NewService(cfg *config.GoogleOauthProvider) *Service {
	return &Service{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint:     google.Endpoint,
		},
		googleUserInfoURL: cfg.GoogleUserInfoURL,
		httpClient:        xhttp.NewClient(),
	}
}
