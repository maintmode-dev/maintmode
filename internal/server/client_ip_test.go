package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
)

// newAPIServerEcho returns the Echo instance NewAPIServer builds, with no
// routes bound -- enough to resolve a request's RealIP the way production does.
func newAPIServerEcho() *echo.Echo {
	return NewAPIServer(config.HTTPServer{}, APIServerHandlers{}, APIServerSecurity{}, nil, false).Echo()
}

// TestAPIServerRealIP pins whose address c.RealIP() names. Audit rows, the
// per-IP rate limiter and the request log all read it, and every caller of
// this service connects through a proxy, so the connecting address is the same
// container for everyone and cannot be the answer.
//
// The public-peer case is the security half: X-Forwarded-For is believed only
// from internal hops, so a caller on a public address cannot name someone
// else.
func TestAPIServerRealIP(t *testing.T) {
	t.Parallel()

	e := newAPIServerEcho()

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{
			name:       "public client behind the gateway",
			remoteAddr: "172.18.0.3:41000",
			xff:        "203.0.113.7",
			want:       "203.0.113.7",
		},
		{
			name:       "client behind the frontend server and an internal proxy",
			remoteAddr: "172.18.0.3:41000",
			xff:        "203.0.113.7, 172.18.0.5",
			want:       "203.0.113.7",
		},
		{
			// A single-host stand: the browser sits on the docker bridge, so
			// every hop is private and the gateway's entry is the client.
			name:       "client on the gateway's own private network",
			remoteAddr: "172.24.0.3:41000",
			xff:        "172.20.0.1",
			want:       "172.20.0.1",
		},
		{
			name:       "public peer cannot claim another address",
			remoteAddr: "198.51.100.9:41000",
			xff:        "203.0.113.7",
			want:       "198.51.100.9",
		},
		{
			name:       "public peer cannot hide behind an internal-looking claim",
			remoteAddr: "198.51.100.9:41000",
			xff:        "10.0.0.1",
			want:       "198.51.100.9",
		},
		{
			name:       "no header means the peer",
			remoteAddr: "172.18.0.3:41000",
			want:       "172.18.0.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/api/v1/login/password", http.NoBody)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set(echo.HeaderXForwardedFor, tt.xff)
			}

			require.Equal(t, tt.want, e.NewContext(req, httptest.NewRecorder()).RealIP())
		})
	}
}
