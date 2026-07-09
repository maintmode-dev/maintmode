package integration

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// GetByKind returns the masked integration for a kind (ErrIntegrationNotFound if
// none). Secrets are never surfaced as plaintext or ciphertext — only is-set.
func (s *Service) GetByKind(ctx context.Context, kind string) (*entity.MaskedIntegration, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.GetByKind",
		xfield.String("kind", kind),
	)
	defer span.End()

	setting, err := s.store.GetByKind(ctx, kind)
	if err != nil {
		xlog.Error(ctx, "failed to get integration", xfield.Error(err))
		return nil, err
	}
	return setting.Mask(), nil
}

// List returns all integrations as masked views, ordered by kind.
func (s *Service) List(ctx context.Context) ([]*entity.MaskedIntegration, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.List")
	defer span.End()

	settings, err := s.store.List(ctx)
	if err != nil {
		xlog.Error(ctx, "failed to list integrations", xfield.Error(err))
		return nil, err
	}

	out := make([]*entity.MaskedIntegration, 0, len(settings))
	for _, setting := range settings {
		out = append(out, setting.Mask())
	}
	return out, nil
}
