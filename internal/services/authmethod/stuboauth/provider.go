package stuboauth

import (
	"github.com/ruko1202/maintmode/internal/entity"
)

// Service is the dev-only stand-in for a real OIDC provider. It carries no
// state: the redirect URL it used to hold was written at construction and
// never read, left over from the backend-owned OAuth flow. Unlike
// googleoauth.NewProvider it therefore takes no config.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (p *Service) MethodID() entity.AuthMethod {
	return entity.AuthMethodStub
}
