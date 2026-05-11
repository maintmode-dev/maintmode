package stuboauth

import (
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

type Service struct {
	authRedirectURL string
}

func NewService(cfg *config.StubOauthProvider) *Service {
	return &Service{
		authRedirectURL: cfg.RedirectURL,
	}
}

func (p *Service) ProviderID() entity.OAuthProvider {
	return entity.OAuthProviderStub
}
