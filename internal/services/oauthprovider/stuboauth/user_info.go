package stuboauth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (g *Service) UserInfo(ctx context.Context, _ string) (*entity.OAuthProviderUserInfo, error) {
	_, span := xlog.WithOperationSpan(ctx, "service.OAuth.Stub.UserInfo")
	defer span.End()

	id := xuuid.NewString()
	return &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: fmt.Sprintf("%s@mail.com", id),
		Name:  fmt.Sprintf("User Name[%s]", id),
	}, nil
}
