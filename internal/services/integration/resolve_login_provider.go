package integration

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// ResolveID turns a login provider's system name into the id of the row behind
// it, for the sign-in paths that must store a reference rather than a name.
//
// Scoped to the login category, and that scoping is correctness rather than
// tidiness: integration_settings is unique on (kind, name), so a delivery row
// and a login provider may legitimately share a name. An unscoped lookup could
// hand back the id of a Slack integration and link someone's account to it.
//
// A name with no row is an error, never a silent fallback. The caller is
// choosing which column identifies the method, and demoting an unresolvable
// provider to the built-in branch would write an identity that no provider
// stands behind.
//
// enabled is NOT consulted. Whether a provider may currently sign people in is
// decided upstream, where the snapshot is built; asking again here would be a
// second predicate free to disagree with the first.
func (s *Service) ResolveID(ctx context.Context, name entity.AuthMethod) (uuid.UUID, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.ResolveID",
		xfield.String("name", string(name)),
	)
	defer span.End()

	row, err := s.store.GetByKindName(ctx, integrationkinds.CategoryLogin, string(name))
	if err != nil {
		// Logged here, where the name is known. The caller sees a failed
		// sign-in, and an operator needs to tell "no such provider" apart from
		// "provider disabled" and "bad token" -- all three look identical from
		// outside.
		xlog.Error(ctx, "failed to resolve login provider", xfield.Error(err))

		return uuid.Nil, fmt.Errorf("resolve login provider %q: %w", name, err)
	}

	return row.ID, nil
}
