package emailtransport

import (
	"context"
	"net"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/ruko1202/xhttp/dialguard"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// The SMTP host is typed by an admin, and the probe endpoint dials it on
// request. By default an internal address must be refused before the
// connection is made: the listener below is live, so a dial that got through
// would be accepted and counted.
func TestNew_RefusesAnInternalHostByDefault(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	var accepted atomic.Int64
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed by cleanup
			}
			accepted.Add(1)
			_ = conn.Close()
		}
	}()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	client, err := New(Params{Host: host, Port: port, From: "noreply@maintmode.test", TLSPolicy: "none"})
	require.NoError(t, err)

	_, err = client.Send(context.Background(), "admin@example.com",
		entity.NotifyMessage{Subject: "s", Body: "b", MessageMIME: entity.TextMessageMIME}, nil)

	require.ErrorIs(t, err, dialguard.ErrBlockedAddress)
	require.Zero(t, accepted.Load())
}
