package oauth2

import (
	"context"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Vendor is the half of an OAuth 2.0 sign-in that is NOT interchangeable.
//
// The authorization-code grant is the same everywhere, which is why the client
// implements it once. "Who does this access token belong to" is not: it is
// answered by a different endpoint, with a different payload, under a different
// verified-email rule at every vendor. A Vendor is exactly that difference, and
// nothing else belongs in one.
type Vendor interface {
	// DefaultScopes is what a sign-in at this vendor needs when the operator
	// configures none.
	//
	// The vendor owns it because it is the vendor that knows which scope makes
	// its identity endpoints readable, and requesting more than that widens the
	// consent screen the user reads while granting access to nothing.
	DefaultScopes() []string

	// ResolveIdentity reports the account an access token belongs to.
	//
	// A vendor makes its own calls: which endpoints answer, how many of them,
	// what they return and which address may be trusted are one decision, and
	// splitting it across a shared helper would only move GitHub's choices into
	// a contract every other vendor has to carry.
	//
	// The TOTAL budget is not a vendor's to set. The caller bounds ctx before
	// handing it over, and context.WithTimeout only ever tightens a deadline --
	// so a vendor may bound one call more tightly, never the set of them more
	// loosely.
	ResolveIdentity(ctx context.Context, accessToken string) (*entity.OAuth2Identity, error)
}
