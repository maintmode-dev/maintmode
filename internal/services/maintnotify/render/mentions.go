package render

import (
	"context"
	"strings"

	"github.com/ruko1202/maintmode/internal/entity"
)

// mentionsLine renders the additionally mentioned people as one comma-separated
// string for the given transport, or the empty string when there is nobody to
// mention.
//
// Returning a finished string rather than a slice is what keeps the template
// honest: {{if .Mentions}} on a slice is true for a non-empty list whose entries
// all dropped out, which would emit a bare "Mentions:" header with nothing after
// it. The condition has to be evaluated after every filter, and a string is the
// only shape that guarantees it.
//
// Per person this is ownerMention: picking a handle for the transport and
// falling back to the sanitized display name is the same job whether the person
// is the owner or merely tagged. The lists differ before they get here — the
// resolver drops unresolvable and blocked mentions, while the owner keeps a
// label — so by this point there is nothing left to tell apart.
func mentionsLine(ctx context.Context, transport entity.NotifyTransport, mentions []*entity.UserMention) string {
	parts := make([]string, 0, len(mentions))

	for _, mention := range mentions {
		if mention == nil {
			continue
		}

		parts = append(parts, ownerMention(ctx, transport, mention))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, ", ")
}
