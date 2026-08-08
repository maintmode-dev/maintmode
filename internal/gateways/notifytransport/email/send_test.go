package emailtransport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Params carries a secret (Password). Lock in that it defines no Stringer or JSON
// marshaler so it can never be accidentally logged or serialized in full.
func TestParams_HasNoSecretLeakingSerializer(t *testing.T) {
	t.Parallel()

	var p any = Params{Password: "smtp-secret"}
	_, isStringer := p.(fmt.Stringer)
	require.False(t, isStringer, "Params must not implement fmt.Stringer (would risk logging the password)")
	_, isMarshaler := p.(json.Marshaler)
	require.False(t, isMarshaler, "Params must not implement json.Marshaler (would risk serializing the password)")
}

func TestClient_Send_DeliversToSMTPServer(t *testing.T) {
	t.Parallel()

	captured := make(chan capturedMessage, 1)
	host, port := newMockSMTPServer(t, captured)

	client, err := New(Params{
		Host:      host,
		Port:      port,
		From:      "noreply@maintmode.test",
		TLSPolicy: "none", // plaintext: the mock server speaks no TLS
	})
	require.NoError(t, err)

	const target = "invitee@example.com"
	const link = "https://app.test/accept?token=abc"
	res, err := client.Send(context.Background(), target, entity.NotifyMessage{
		Subject:     "You've been invited",
		Body:        `<p>Accept your invitation: <a href="` + link + `">here</a></p>`,
		MessageMIME: entity.HTMLMessageMIME,
	}, nil)
	require.NoError(t, err)
	// E-mail has no addressable message to thread under, so it never reports a
	// ref — callers must not record a thread root for this transport.
	require.Nil(t, res.Ref)
	require.False(t, res.RootReplaced)

	msg := <-captured
	require.Equal(t, "noreply@maintmode.test", msg.from)
	require.Contains(t, msg.rcpts, target)
	require.Contains(t, msg.data, "Subject: You've been invited")
	require.Contains(t, msg.data, "text/html")
	// Plain-text alternative is attached so non-HTML clients still see content.
	require.Contains(t, msg.data, "text/plain")

	// The body is quoted-printable encoded on the wire, so '=' becomes '=3D'.
	// Decode the DATA payload before asserting the link survived intact.
	decoded := decodeQuotedPrintable(t, msg.data)
	require.Contains(t, decoded, link, "link must be present in the HTML body")
	// The plain-text alternative must keep the link too — anchors are expanded to
	// "text (url)" so a non-HTML client still has a way to accept the invitation.
	require.Contains(t, decoded, "here ("+link+")", "plain-text alternative must keep the link")
	// HTML bodies are wrapped in the branded frame: the wordmark and the default
	// footer line appear in both the HTML and its derived plain text.
	require.Contains(t, decoded, "MaintMode", "wrapped HTML must carry the wordmark")
	require.Contains(t, decoded, defaultFooterText, "wrapped HTML must carry the footer line")
}

func TestClient_Send_PlainTextIsNotWrapped(t *testing.T) {
	t.Parallel()

	captured := make(chan capturedMessage, 1)
	host, port := newMockSMTPServer(t, captured)

	client, err := New(Params{
		Host:      host,
		Port:      port,
		From:      "noreply@maintmode.test",
		TLSPolicy: "none",
	})
	require.NoError(t, err)

	// A plain-text message passes through the transport unframed: no wordmark, no
	// footer, no table markup — only what the caller sent.
	_, err = client.Send(context.Background(), "someone@example.com", entity.NotifyMessage{
		Subject:     "Maintenance started",
		Body:        "Maintenance started for Database.",
		MessageMIME: entity.TextMessageMIME,
	}, nil)
	require.NoError(t, err)

	msg := <-captured
	decoded := decodeQuotedPrintable(t, msg.data)
	require.Contains(t, decoded, "Maintenance started for Database.")
	require.NotContains(t, decoded, "MaintMode", "plain-text messages must not be wrapped in the branded frame")
	require.NotContains(t, decoded, "<table", "plain-text messages must not gain HTML markup")
	require.NotContains(t, decoded, defaultFooterText)
}

func TestClient_New_EmptyHost(t *testing.T) {
	t.Parallel()

	// Empty host fails fast at construction: when email is enabled but no host is
	// configured (e.g. a missing secret), the binary should fail to start rather
	// than silently never delivering.
	_, err := New(Params{From: "noreply@maintmode.test", Host: ""})
	require.Error(t, err)
}

func TestClient_New_EmptyFrom(t *testing.T) {
	t.Parallel()

	// Empty From fails fast too: From is required when enabled, and go-mail rejects
	// an empty From only at send time, which is the silent-until-first-send failure
	// the fail-fast construction exists to avoid.
	_, err := New(Params{Host: "smtp.example.com"})
	require.Error(t, err)
}

func TestHTMLToText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "anchor expands to text (url)",
			in:   `<p>Click <a href="https://x.test/a?t=1">here</a></p>`,
			want: "Click here (https://x.test/a?t=1)",
		},
		{
			name: "entities are unescaped",
			in:   `<a href="https://x.test/a?t=1&amp;u=2">go</a>`,
			want: "go (https://x.test/a?t=1&u=2)",
		},
		{
			name: "br and block tags become newlines",
			in:   "<p>one</p><div>two</div>three<br>four",
			want: "one\n\ntwo\nthree\nfour",
		},
		{
			name: "plain text passes through",
			in:   "just text",
			want: "just text",
		},
		{
			name: "table rows become newlines",
			in:   "<table><tr><td>MaintMode</td></tr><tr><td>body</td></tr><tr><td>footer</td></tr></table>",
			want: "MaintMode\nbody\nfooter",
		},
		{
			name: "runs of blank lines collapse to one",
			in:   "<p>one</p><tr></tr><tr></tr><p>two</p>",
			want: "one\n\ntwo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, htmlToText(tc.in))
		})
	}
}

type capturedMessage struct {
	from  string
	rcpts []string
	data  string
}

// decodeQuotedPrintable decodes the quoted-printable runs in raw SMTP DATA so
// assertions can match the original body. It is line-based and tolerant: the raw
// data interleaves MIME headers and boundaries (not QP) with QP-encoded bodies,
// and quotedprintable.Reader passes plain lines through unchanged.
func decodeQuotedPrintable(t *testing.T, raw string) string {
	t.Helper()
	out, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(raw)))
	require.NoError(t, err)
	return string(out)
}

// newMockSMTPServer stands up an in-process SMTP listener that speaks just enough
// of the protocol (EHLO/MAIL/RCPT/DATA/QUIT) to accept one message, then sends
// the captured envelope on c. It is the SMTP analog of the httptest mock used
// for the slack/telegram transports. Returns the host and port to point a client
// at via Params with TLSPolicy "none".
func newMockSMTPServer(t *testing.T, c chan<- capturedMessage) (host string, port int) {
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

		_ = handleSMTPConn(conn, c)
	}()

	var portStr string
	host, portStr, err = net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	port, err = strconv.Atoi(portStr)
	require.NoError(t, err)

	return host, port
}

// handleSMTPConn runs a single SMTP exchange against conn, capturing the
// envelope. It intentionally implements only the commands go-mail issues for a
// plaintext, no-auth send.
func handleSMTPConn(conn net.Conn, c chan<- capturedMessage) error {
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

	var msg capturedMessage
	inData := false
	var data strings.Builder

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}

		if inData {
			if strings.TrimRight(line, "\r\n") == "." {
				inData = false
				msg.data = data.String()
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
			// Advertise no extensions: keeps go-mail on the plaintext path.
			if err := write("250 mock.smtp.test"); err != nil {
				return err
			}
		case strings.HasPrefix(upper, "MAIL FROM:"):
			msg.from = extractAddr(cmd[len("MAIL FROM:"):])
			if err := write("250 2.1.0 OK"); err != nil {
				return err
			}
		case strings.HasPrefix(upper, "RCPT TO:"):
			msg.rcpts = append(msg.rcpts, extractAddr(cmd[len("RCPT TO:"):]))
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
			c <- msg
			return nil
		default:
			if err := write("250 2.0.0 OK"); err != nil {
				return err
			}
		}
	}
}

// extractAddr pulls the bare address out of an SMTP path argument like
// " <noreply@maintmode.test>" or "<noreply@maintmode.test> SIZE=123".
func extractAddr(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '<'); i >= 0 {
		s = s[i+1:]
		if j := strings.IndexByte(s, '>'); j >= 0 {
			s = s[:j]
		}
	} else if i := strings.IndexByte(s, ' '); i >= 0 {
		s = s[:i]
	}
	return s
}
