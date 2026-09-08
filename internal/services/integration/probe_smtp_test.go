package integration_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// The probe is the one endpoint that opens a connection on the caller's say-so
// and reports what happened, so these tests pin the two things an admin
// depends on: a real send when the settings work, and a truthful, categorized
// failure when they do not.
func TestProbeEmail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("a working config delivers and reports success", func(t *testing.T) {
		t.Parallel()

		srv, kinds, _ := initService(t)
		captured := make(chan capturedEnvelope, 1)
		host, port := newMockSMTPServer(t, captured)

		err := srv.Probe(ctx, &entity.ProbeIntegrationCmd{
			Kind:    kinds.email,
			Config:  emailConfig(t, host, port, nil),
			Secrets: map[string]string{},
			To:      "admin@example.com",
			Actor:   testActor(),
		})
		require.NoError(t, err)

		envelope := <-captured
		require.Equal(t, []string{"admin@example.com"}, envelope.rcpts)
		require.Contains(t, envelope.data, "MaintMode")
	})

	t.Run("an unreachable host fails with the dial error", func(t *testing.T) {
		t.Parallel()

		srv, kinds, _ := initService(t)

		// Port 1 on loopback: nothing listens, so the dial is refused promptly
		// rather than hanging on a firewall drop.
		err := srv.Probe(ctx, &entity.ProbeIntegrationCmd{
			Kind:    kinds.email,
			Config:  emailConfig(t, "127.0.0.1", 1, nil),
			Secrets: map[string]string{},
			To:      "admin@example.com",
			Actor:   testActor(),
		})
		require.ErrorIs(t, err, apperr.ErrIntegrationProbeFailed)
		require.ErrorContains(t, err, "connect")
	})

	t.Run("settings the kind rejects never reach the transport", func(t *testing.T) {
		t.Parallel()

		srv, kinds, _ := initService(t)
		host, port := newMockSMTPServer(t, make(chan capturedEnvelope, 1))

		// A username with no password, aimed at a server that would otherwise
		// answer. The kind requires the two together; the transport does not
		// care. Pointing this at a live server is what makes the case pin
		// Validate: against a dead port the same assertion would pass on a dial
		// error and prove nothing.
		err := srv.Probe(ctx, &entity.ProbeIntegrationCmd{
			Kind:    kinds.email,
			Config:  emailConfig(t, host, port, map[string]any{"username": "smtp-user"}),
			Secrets: map[string]string{},
			To:      "admin@example.com",
			Actor:   testActor(),
		})
		require.ErrorIs(t, err, apperr.ErrValidation)
		require.ErrorContains(t, err, "username and password must be set together")
		// Not merely "an error": without Validate the transport dials and comes
		// back with a probe failure, i.e. a 502 where the admin deserves a 400
		// naming the field.
		require.NotErrorIs(t, err, apperr.ErrIntegrationProbeFailed)
	})

	t.Run("a secret sent in the config section is refused", func(t *testing.T) {
		t.Parallel()

		srv, kinds, _ := initService(t)
		host, port := newMockSMTPServer(t, make(chan capturedEnvelope, 1))

		err := srv.Probe(ctx, &entity.ProbeIntegrationCmd{
			Kind:    kinds.email,
			Config:  emailConfig(t, host, port, map[string]any{"password": "in-the-wrong-section"}),
			Secrets: map[string]string{},
			To:      "admin@example.com",
			Actor:   testActor(),
		})
		require.ErrorIs(t, err, apperr.ErrValidation)
		require.ErrorContains(t, err, "password")
	})

	// Credentials must never be offered over a channel the caller asked to leave
	// unencrypted. go-mail narrows auto-discovery to challenge-response
	// mechanisms when there is no TLS, so the password itself does not cross the
	// wire in the clear -- but a hostile host advertising CRAM-MD5 still walks
	// away with a challenge/response pair that brute-forces offline. Since the
	// host is caller-supplied, that is a credential-exposure path, distinct from
	// the SSRF risk that was accepted deliberately.
	t.Run("credentials are refused over an unencrypted channel", func(t *testing.T) {
		t.Parallel()

		srv, kinds, _ := initService(t)
		host, port := newMockSMTPServer(t, make(chan capturedEnvelope, 1))

		err := srv.Probe(ctx, &entity.ProbeIntegrationCmd{
			Kind:    kinds.email,
			Config:  emailConfig(t, host, port, map[string]any{"username": "smtp-user"}),
			Secrets: map[string]string{"password": "s3cr3t"},
			To:      "admin@example.com",
			Actor:   testActor(),
		})
		require.ErrorIs(t, err, apperr.ErrValidation)
		require.ErrorContains(t, err, "tls_policy")
	})

	// The same config WITHOUT credentials stays allowed: an unauthenticated
	// relay over plaintext exposes nothing, and it is what the in-process test
	// server speaks.
	t.Run("an anonymous relay over plaintext is still allowed", func(t *testing.T) {
		t.Parallel()

		srv, kinds, _ := initService(t)
		captured := make(chan capturedEnvelope, 1)
		host, port := newMockSMTPServer(t, captured)

		require.NoError(t, srv.Probe(ctx, &entity.ProbeIntegrationCmd{
			Kind:    kinds.email,
			Config:  emailConfig(t, host, port, nil),
			Secrets: map[string]string{},
			To:      "admin@example.com",
			Actor:   testActor(),
		}))
		<-captured
	})

	t.Run("a bad recipient is rejected before any connection", func(t *testing.T) {
		t.Parallel()

		srv, kinds, _ := initService(t)
		host, port := newMockSMTPServer(t, make(chan capturedEnvelope, 1))

		for _, to := range []string{"", "not-an-address", "a@b.c\r\nBcc: c@d.e"} {
			err := srv.Probe(ctx, &entity.ProbeIntegrationCmd{
				Kind:    kinds.email,
				Config:  emailConfig(t, host, port, nil),
				Secrets: map[string]string{},
				To:      to,
				Actor:   testActor(),
			})
			require.ErrorIs(t, err, apperr.ErrValidation, "to=%q", to)
		}
	})

	t.Run("a negative timeout is rejected", func(t *testing.T) {
		t.Parallel()

		srv, kinds, _ := initService(t)
		host, port := newMockSMTPServer(t, make(chan capturedEnvelope, 1))

		// ParseDuration accepts it and the kind's Validate does not object, but
		// a negative deadline expires instantly -- a 502 against a healthy
		// server.
		err := srv.Probe(ctx, &entity.ProbeIntegrationCmd{
			Kind:    kinds.email,
			Config:  emailConfig(t, host, port, map[string]any{"timeout": "-5m"}),
			Secrets: map[string]string{},
			To:      "admin@example.com",
			Actor:   testActor(),
		})
		require.ErrorIs(t, err, apperr.ErrValidation)
	})

	// The password reaches the transport, so it could reach an error string.
	// Asserting its absence beats assuming it: if a library upgrade ever echoed
	// the credential, this fails instead of the leak shipping.
	t.Run("a failure never echoes the password", func(t *testing.T) {
		t.Parallel()

		srv, kinds, _ := initService(t)
		const password = "s3cr3t-must-not-appear"

		err := srv.Probe(ctx, &entity.ProbeIntegrationCmd{
			Kind:    kinds.email,
			Config:  emailConfig(t, "127.0.0.1", 1, map[string]any{"username": "smtp-user"}),
			Secrets: map[string]string{"password": password},
			To:      "admin@example.com",
			Actor:   testActor(),
		})
		require.Error(t, err)
		require.NotContains(t, err.Error(), password)
	})
}

type capturedEnvelope struct {
	rcpts []string
	data  string
}

// emailConfig builds a kind config aimed at host:port. Extra keys are merged in
// last, so a case can add "timeout" or "username" without a variant helper.
func emailConfig(t *testing.T, host string, port int, extra map[string]any) json.RawMessage {
	t.Helper()

	cfg := map[string]any{
		"host":       host,
		"port":       port,
		"from":       "noreply@maintmode.test",
		"tls_policy": "none", // the mock server speaks no TLS
	}
	for k, v := range extra {
		cfg[k] = v
	}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	return raw
}

// newMockSMTPServer answers one plaintext SMTP conversation and reports the
// envelope it saw. It implements only the commands go-mail issues on the
// no-auth path.
func newMockSMTPServer(t *testing.T, c chan<- capturedEnvelope) (host string, port int) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed by cleanup
		}
		defer conn.Close()
		_ = handleSMTP(conn, c)
	}()

	var portStr string
	host, portStr, err = net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	port, err = strconv.Atoi(portStr)
	require.NoError(t, err)

	return host, port
}

func handleSMTP(conn net.Conn, c chan<- capturedEnvelope) error {
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	write := func(s string) error {
		if _, err := w.WriteString(s + "\r\n"); err != nil {
			return err
		}
		return w.Flush()
	}

	if err := write("220 mock.smtp.test ESMTP ready"); err != nil {
		return err
	}

	var (
		envelope capturedEnvelope
		data     strings.Builder
		inData   bool
	)

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}

		if inData {
			if strings.TrimRight(line, "\r\n") == "." {
				inData = false
				envelope.data = data.String()
				if err := write("250 2.0.0 OK"); err != nil {
					return err
				}
				continue
			}
			data.WriteString(line)
			continue
		}

		cmd := strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(cmd)

		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			if err := write("250 mock.smtp.test"); err != nil {
				return err
			}
		case strings.HasPrefix(upper, "RCPT TO:"):
			envelope.rcpts = append(envelope.rcpts, extractAddress(cmd[len("RCPT TO:"):]))
			if err := write("250 2.1.5 OK"); err != nil {
				return err
			}
		case upper == "DATA":
			inData = true
			if err := write("354 End data with <CR><LF>.<CR><LF>"); err != nil {
				return err
			}
		case upper == "QUIT":
			_ = write("221 2.0.0 Bye")
			c <- envelope
			return nil
		default:
			if err := write("250 2.0.0 OK"); err != nil {
				return err
			}
		}
	}
}

func extractAddress(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<")
	if i := strings.IndexByte(s, '>'); i >= 0 {
		s = s[:i]
	}
	return s
}
