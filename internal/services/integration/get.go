package integration

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// GetByKindName returns the masked integration for one (kind, name)
// (ErrIntegrationNotFound if none). Secrets are never surfaced as plaintext or
// ciphertext — only is-set.
func (s *Service) GetByKindName(ctx context.Context, kind, name string) (*entity.MaskedIntegration, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.GetByKindName",
		xfield.String("kind", kind),
		xfield.String("name", name),
	)
	defer span.End()

	setting, err := s.store.GetByKindName(ctx, kind, name)
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
