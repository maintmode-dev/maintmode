package stuboauth

import "github.com/ruko1202/maintmode/internal/config"

type Service struct {
	authRedirectURL string
}

func NewService(cfg *config.StubOauthProvider) *Service {
	return &Service{
		authRedirectURL: cfg.RedirectURL,
	}
}
