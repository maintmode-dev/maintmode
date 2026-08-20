// Package license is the HTTP gateway to the Console license server: one POST
// per tick carries the instance report and brings back the license. There is
// no retry — the heartbeat cadence is the retry, and every failure is
// fail-open (the caller keeps the cached license).
//
// This path serves the paid seat-based SaaS offering only. Nothing here runs
// unless config.LicenseConfig.Enabled() is true, which requires BOTH the
// Console url and an instance_token; a half-set block stays off. Self-hosted
// deployments set neither (the sample configs under deployment/ ship no
// license block at all), so bootstrap wires license.Noop instead: the
// heartbeat processor is never registered, this client is never constructed,
// no seat cap is enforced and no request ever leaves the instance. The code
// ships in the public repo because both offerings run the same binary.
package license

import (
	"cmp"
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ruko1202/xhttp/client"

	"github.com/ruko1202/maintmode/internal/utils/xsanitize"

	"github.com/ruko1202/maintmode/internal/config"
)

const heartbeatPath = "/cloud/v1/instances/heartbeat"

// maxResponseBytes bounds the heartbeat response body: the real payload is a
// few hundred bytes, so anything near the limit is a misconfigured URL
// serving something else entirely.
const maxResponseBytes = 1 << 20

// Client talks to the Console license server.
type Client struct {
	baseURL string
	httpc   *http.Client
}

// NewClient builds the heartbeat client. A non-positive timeout keeps the
// library default. The base URL is normalized (trailing slash stripped) so a
// config value like "https://console.example.com/" cannot produce a
// //cloud/... path that 404s on every tick forever.
func NewClient(cfg config.LicenseConfig) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.URL, "/"),
		httpc: client.NewClient(
			client.WithTimeout(cmp.Or(cfg.HTTPTimeout, time.Second)),
			client.WithSanitizer(xsanitize.New()),
			// The library ships no bearer helper: which credential a service
			// presents is its own concern, not the transport's.
			client.WithCallerBeforeDo(func(_ context.Context, req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+cfg.InstanceToken)
			}),
		),
	}
}
