package xvalidation

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// refuseInternalHost rejects a URL pointing at an address only the server can
// reach.
//
// Unexported, and part of HTTPSURL rather than a rule of its own. It was its
// own package while two callers needed it at two different moments -- the
// issuer an operator types and the endpoints a discovery document names -- but
// both now go through HTTPSURL, and a URL that is safe to store is one that is
// https, carries no credential AND reaches nothing internal. Offering the
// address half separately would let a caller take it and forget the other two.
//
// This is an SSRF control, and the move of login providers into the registry is
// what makes it necessary. While providers lived in the config file, issuer_url
// could only be set by someone with shell access and a restart; configuring one
// through the API puts it within reach of the admin role, and the server then
// fetches {issuer_url}/.well-known/openid-configuration on its own behalf.
// Without this an admin could aim that fetch at 169.254.169.254 and read cloud
// instance credentials out of the error, or sweep loopback and private ranges
// for services that trust the network rather than the caller.
//
// It applies to the endpoints the DISCOVERY DOCUMENT names as well, and there
// the reasoning is sharper: those URLs are chosen by whoever serves the
// document rather than by the operator, and the client secret goes to the token
// endpoint while the signing keys that decide whether a token is genuine come
// from the JWKS one.
//
// Literal addresses only. A hostname that RESOLVES to a private address is not
// caught here and cannot be, short of resolving at validation time and again at
// fetch time -- two lookups that can disagree, which is the rebinding attack
// rather than a defense against it. What this does close is the direct form,
// which is the one that needs no setup at all.
func refuseInternalHost(raw string) error {
	// A URL that will not parse is not this rule's business: is.URL and the
	// https check beside it already refuse it, and reporting the same input
	// twice would give the operator two errors for one mistake.
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil //nolint:nilerr // shape is judged by the rules beside this one
	}
	if parsed.Host == "" {
		return nil
	}

	host := parsed.Hostname()

	// Named loopback resolves to a literal loopback everywhere that matters,
	// and spelling it out is how the obvious attempt is written.
	if strings.EqualFold(host, "localhost") {
		return errors.New("must not point at localhost")
	}

	// A host made only of digits and dots -- or of hex behind an 0x prefix -- is
	// an IP literal written in one of the spellings net.ParseIP refuses and the
	// resolver accepts: 2130706433, 0x7f000001, 017700000001, 127.1 and
	// 0177.0.0.1 all reach 127.0.0.1. Judged by shape rather than resolved; see
	// isNumericHost for why that is the safe direction to be wrong in.
	if isNumericHost(host) {
		return errors.New("must not point at a numeric host")
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return nil // a name: see the doc comment on why this is where it stops
	}

	switch {
	case ip.IsLoopback():
		return errors.New("must not point at a loopback address")
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		// 169.254.0.0/16 and fe80::/10 -- where cloud metadata lives.
		return errors.New("must not point at a link-local address")
	case ip.IsPrivate():
		return errors.New("must not point at a private address")
	case ip.IsUnspecified():
		return errors.New("must not point at an unspecified address")
	case ip.IsMulticast():
		return errors.New("must not point at a multicast address")
	case inReservedRange(ip):
		// net.IP.IsPrivate covers RFC1918 and fc00::/7 and stops there, which
		// leaves out the range that matters most here: 100.64.0.0/10 is the
		// carrier-grade NAT block several clouds use as their internal service
		// network, and 100.100.100.200 is Alibaba's metadata endpoint.
		return errors.New("must not point at a reserved address")
	}

	return nil
}

// reservedRanges are the special-purpose blocks net.IP's own predicates miss.
//
// Kept as an explicit list rather than folded into the switch above: each entry
// is a separate factual claim about the internet, and a reader checking one of
// them should not have to read a boolean expression to do it.
//
// THE v4 ENTRIES ARE UNREACHABLE BY BEHAVIOR, and that is worth stating so the
// next reader does not discover it by deleting one. isNumericHost refuses any
// host made of digits and dots, which is every dotted quad, so a v4 literal
// never reaches these ranges: 100.64.0.1 is refused as a numeric host one
// branch earlier. They are kept anyway, because the shape rule refuses those
// hosts for being SPELLED like an address and this list for BEING one, and the
// second reason is what survives if the first ever narrows. The v6 entries are
// live -- a bracketed v6 literal is not digits-and-dots, so it reaches this
// list and nothing else would refuse it.
//
// Changing the list is still a falsifiable act: TestReservedRanges_Composition
// asserts the whole slice, so an addition or a removal fails there even when no
// address changes verdict. That test is the reason a v4 entry can be kept
// without becoming a line nobody dares touch.
//
// 0.0.0.0/8 is deliberately absent, and TestReservedRanges_OmitsThisNetwork
// records why: unlike the v4 entries above it would add no coverage at all,
// since its v6 spellings are already held by the translated and 6to4 prefixes.
var reservedRanges = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),   // carrier-grade NAT; cloud internal networks
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1
	netip.MustParsePrefix("192.88.99.0/24"),  // 6to4 relay anycast
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved for future use

	// The v6 spellings of a v4 destination. Each reaches the address it names
	// while every net.IP predicate stays silent, so each needs its own line.
	netip.MustParsePrefix("64:ff9b::/96"),   // NAT64 well-known, which can carry a v4 target
	netip.MustParsePrefix("64:ff9b:1::/48"), // NAT64 local-use (RFC 8215): the operator's OWN translator
	// v4-TRANSLATED (RFC 2765), and the trap is the prefix itself: this is NOT
	// ::ffff:0:0/96, which is the v4-MAPPED form net.ParseIP already folds to
	// four bytes. On a translated address Is4In6() is false and Unmap() is a
	// no-op, so ::ffff:0:7f00:1 is loopback that no predicate recognizes.
	netip.MustParsePrefix("::ffff:0:0:0/96"),
	netip.MustParsePrefix("2002::/16"), // 6to4: 2002:7f00:1::1 is 127.0.0.1
	// Site-local. Deprecated by RFC 3879 and still routed, and it is not
	// link-local: fe80::/10 ends at febf::, so IsLinkLocalUnicast misses it.
	netip.MustParsePrefix("fec0::/10"),
}

func inReservedRange(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}

	addr = addr.Unmap()
	for _, prefix := range reservedRanges {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

// isNumericHost reports whether a host is an IP literal spelled so that
// net.ParseIP will not recognize it.
//
// The test is "digits, dots and an optional 0x prefix, and nothing else",
// which is broader than it first looks. inet_aton accepts an address in parts
// -- "127.1" fills the remaining 24 bits from the last part -- and reads a
// part with a leading zero as octal, so 127.0.0.1 can be written 2130706433,
// 0x7f000001, 017700000001, 127.1 and 0177.0.0.1, and the resolver maps every
// one of them home. net.ParseIP accepts only the canonical dotted quad and
// returns nil for the rest, which is the gap this closes: without it those
// spellings fall through to "not an IP literal, so it must be a name" and are
// waved past.
//
// Matching by SHAPE rather than by resolving is deliberate, for the reason the
// doc comment on RefuseInternalHost gives -- a lookup here and another at fetch
// time can disagree. The shape is safe to refuse wholesale because a host made
// only of digits and dots is never a legitimate issuer: either it parses as an
// address, and the predicates below judge it, or it is one of these spellings.
//
// A canonical dotted quad also matches here, and that is harmless -- it would
// be refused a few lines further down anyway, and carving it out would mean
// re-implementing net.ParseIP to decide which of the two errors to give.
func isNumericHost(host string) bool {
	if host == "" {
		return false
	}

	if lower := strings.ToLower(host); strings.HasPrefix(lower, "0x") {
		// Hex needs its own pass: the digits below stop at '9'.
		return isHexHost(lower[len("0x"):])
	}

	for _, r := range host {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}

	return true
}

// isHexHost reports whether what follows a 0x prefix is a hex number, and not
// merely a name that happens to start with those two letters. "0xdeadbeef" is
// 3735928559; "0x-tools.example" is a hostname.
func isHexHost(digits string) bool {
	if digits == "" {
		return false
	}

	for _, r := range digits {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f':
		default:
			return false
		}
	}

	return true
}
