package server

import (
	"github.com/labstack/echo/v5"
	xhttpserver "github.com/ruko1202/xhttp/server"
)

// clientIPExtractor resolves c.RealIP() to the client behind the proxies rather
// than to the connecting peer.
//
// No caller of this service connects directly: the browser reaches it through
// the gateway (the OAuth dance) or through the frontend server, which relays
// the gateway's X-Forwarded-For verbatim. Without an extractor RealIP is that
// proxy's address, so every audit row, every per-IP rate-limit bucket and every
// request log line names the same container instead of the person.
//
// Trust assumption, and it is the whole security argument: echo's defaults
// trust an X-Forwarded-For hop only when it is loopback, link-local or
// private (RFC 1918 / RFC 4193). The header is walked from the connecting peer
// outwards and the first address outside those ranges wins, so:
//
//   - a peer with a public address is answered with its own address, whatever
//     header it sends -- the internet cannot claim to be someone else;
//   - a peer on an internal network IS believed. That is safe only because the
//     outermost proxy -- the gateway -- overwrites X-Forwarded-For with the
//     address it saw rather than appending to what the client sent, and because
//     this service is reachable on internal networks only. Publishing the
//     backend port, or putting a proxy in front that passes the client's header
//     through, makes the recorded address client-chosen.
//
// When every hop is internal (a client on the same private network as the
// gateway, as on a single-host stand), the leftmost entry is returned: the
// address the gateway wrote.
var clientIPExtractor = echo.ExtractIPFromXFFHeader()

// withClientIPExtractor installs clientIPExtractor on the server's Echo. It is
// applied after the caller's options so a WithEcho among them cannot replace
// the instance it was set on.
func withClientIPExtractor() xhttpserver.Option {
	return func(s *xhttpserver.Server) {
		s.Echo().IPExtractor = clientIPExtractor
	}
}
