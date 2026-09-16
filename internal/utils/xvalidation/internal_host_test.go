package xvalidation

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The spellings of an address that net.ParseIP refuses and the resolver
// accepts. A guard that stops at "net.ParseIP said nil, so it must be a name"
// waves every one of these through to a fetch that lands on 127.0.0.1.
//
// Tested HERE rather than through the issuer validation that calls it, and the
// distinction is the point: over there is.URL runs first and rejects "127.1"
// and "0177.0.0.1" as malformed, so those cases pass whether this guard looks
// at them or not. Mutating the guard leaves that suite green. Only a test
// against the guard itself can fail when the guard stops working.
func TestRefuseInternalHost_RefusesNonCanonicalSpellings(t *testing.T) {
	t.Parallel()

	refused := map[string]string{
		"32-bit integer":    "https://2130706433/idp",
		"hex":               "https://0x7f000001/idp",
		"octal":             "https://017700000001/idp",
		"two-part short":    "https://127.1/idp",
		"dotted with octal": "https://0177.0.0.1/idp",

		// The canonical spellings, which the address predicates judge. Here to
		// pin that widening the shape test did not cost us the plain cases.
		"dotted quad":    "https://127.0.0.1/idp",
		"cloud metadata": "https://169.254.169.254/latest/meta-data",
		"named loopback": "https://localhost/idp",

		// The v6 spellings of a v4 destination. Every one of these reached the
		// address it names while net.IP's predicates said nothing, so they are
		// the same class as the inet_aton forms above, arriving through a
		// different door.
		//
		// v4-mapped already worked before these ranges were added, and it is
		// here to keep it that way: net.ParseIP folds ::ffff:127.0.0.1 to the
		// 4-byte 127.0.0.1 on its own, so IsLoopback and IsPrivate judge it
		// like any dotted quad. Nothing in the guard mentions the mapped form,
		// which is exactly why a test has to.
		"v4-mapped loopback": "https://[::ffff:127.0.0.1]/idp",
		"v4-mapped private":  "https://[::ffff:10.0.0.1]/idp",
		// v4-TRANSLATED (RFC 2765) is a different prefix and behaves
		// differently: Is4In6() is false, Unmap() is a no-op, ::/96 does not
		// contain it, and no stdlib predicate catches it. It needs its own
		// entry in reservedRanges, and ::ffff:0:0/96 is NOT that entry -- that
		// one is the mapped form spelled another way.
		"v4-translated loopback": "https://[::ffff:0:7f00:1]/idp",
		"v4-translated metadata": "https://[::ffff:0:a9fe:a9fe]/latest/meta-data",
		// NAT64 local-use (RFC 8215): the range an operator picks for their OWN
		// translator, so the well-known 64:ff9b::/96 alone leaves it open.
		"nat64 local-use metadata": "https://[64:ff9b:1::a9fe:a9fe]/latest/meta-data",
		// Site-local. Deprecated by RFC 3879 and still routed by stacks, and
		// fe80::/10 stops at febf:: so IsLinkLocalUnicast misses it.
		"site-local": "https://[fec0::1]/idp",
		// 6to4 embeds a v4 address in the second and third groups:
		// 2002:7f00:1::1 is 127.0.0.1.
		"6to4 loopback": "https://[2002:7f00:1::1]/idp",
		// IsUnspecified covers 0.0.0.0 and nothing else, but the whole /8 is
		// "this network" and 0.0.0.1 reaches loopback on Linux.
		//
		// Every spelling of it is already refused by something else -- the
		// dotted form by the digits-and-dots shape rule, the v6 forms by the
		// translated and 6to4 prefixes -- which is why reservedRanges carries
		// no 0.0.0.0/8 entry (see TestReservedRanges_OmitsThisNetwork). These
		// cases pin the ADDRESS being refused, not which rule refuses it.
		"this-network":               "https://0.0.0.1/idp",
		"this-network v4-translated": "https://[::ffff:0:0:1]/idp",
		"this-network 6to4":          "https://[2002:0:1::1]/idp",
	}

	for name, raw := range refused {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()

			require.Error(t, refuseInternalHost(raw),
				"%q reaches an address only the server can see", raw)
		})
	}
}

// The shape test refuses a host made of digits and dots, which is a wide net.
// These are the names it must not catch: a real issuer whose label starts with
// a digit, a subdomain that is only a digit, and a name that begins with the
// two characters the hex branch keys on.
func TestRefuseInternalHost_AllowsPublicIssuers(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"https://accounts.google.com",
		"https://idp.corp.example",
		"https://localhost.example.com",
		"https://1password.com",
		"https://3.basecamp.com",
		"https://0x-tools.example",
	} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, refuseInternalHost(raw),
				"%q is a public issuer and must be allowed", raw)
		})
	}
}
