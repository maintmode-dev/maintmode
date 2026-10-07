// Package emailtransport delivers notifications over SMTP. It implements the
// notify Transport (TransportID + Send) so email lives in the same registry as
// slack/telegram rather than a parallel abstraction.
//
// Real delivery is opt-in via the DB-backed integration registry: an
// admin enables and configures the email transport at runtime. New fails fast
// when the transport is enabled but misconfigured (e.g. an empty host or from),
// so a broken integration surfaces at build time rather than silently never
// delivering. In dev the resolver's stub short-circuit routes deliveries to the
// stub transport instead.
package emailtransport

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/ruko1202/xhttp/dialguard"
	"github.com/wneessen/go-mail"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

const (
	defaultTimeout = 10 * time.Second
	defaultPort    = 587
)

// Params is the constructor input for the email/SMTP transport. It is populated
// from the DB-backed integration settings; Password is a secret and
// the type intentionally has no Stringer/marshaler.
type Params struct {
	Host      string
	Port      int
	Username  string
	Password  string
	From      string
	ReplyTo   string
	TLSPolicy string
	Timeout   time.Duration
	// AllowInternalHosts lifts the dial guard. See
	// config.NotifyTransportConfig.AllowInternalHosts.
	AllowInternalHosts bool
}

type Client struct {
	client  *mail.Client
	from    string
	replyTo string
}

// New constructs the transport from cfg. It returns an error when the transport
// is misconfigured — an empty Host or From — so an enabled-but-misconfigured
// email transport fails at startup instead of silently at first send.
func New(cfg Params) (*Client, error) {
	if cfg.From == "" {
		return nil, fmt.Errorf("email transport: from address is required")
	}

	opts := []mail.Option{
		mail.WithPort(cmp.Or(cfg.Port, defaultPort)),
		mail.WithTimeout(cmp.Or(cfg.Timeout, defaultTimeout)),
		mail.WithTLSPortPolicy(tlsPolicy(cfg.TLSPolicy)),
	}
	// The host is typed by an admin, and the probe endpoint dials it on
	// request, so without a guard the transport is a port scanner for the
	// internal network. The guard judges the resolved address, which a check
	// on the configured string cannot: a public name can resolve inward.
	if !cfg.AllowInternalHosts {
		opts = append(opts, mail.WithDialContextFunc(guardedDial()))
	}
	if cfg.Username != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(cfg.Username),
			mail.WithPassword(cfg.Password),
		)
	}

	c, err := mail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("email client init failed: %w", err)
	}

	return &Client{client: c, from: cfg.From, replyTo: cfg.ReplyTo}, nil
}

// guardedDial is a plain TCP dial that refuses internal addresses after the
// name is resolved and before the connection is made, on every address the
// resolver returned.
//
// It replaces go-mail's default dialer, which would also wrap the connection
// in TLS for implicit-TLS ports. That path is never taken here: the TLS policy
// is always STARTTLS-or-none (WithTLSPortPolicy), never WithSSL.
func guardedDial() mail.DialContextFunc {
	guard := dialguard.Blocking(dialguard.InternalPrefixes()...)
	dialer := &net.Dialer{
		ControlContext: func(_ context.Context, network, address string, _ syscall.RawConn) error {
			return guard(network, address)
		},
	}

	return dialer.DialContext
}

func (*Client) TransportID() entity.NotifyTransport {
	return entity.NotifyTransportEmail
}

// TLS policy config strings. Unknown/empty values default to mandatory STARTTLS —
// the safe production posture. tlsPolicyNone is the plaintext mode used by the
// in-process SMTP test server.

// tlsPolicy maps the config string to a go-mail TLSPolicy.
func tlsPolicy(s string) mail.TLSPolicy {
	switch s {
	case integrationkinds.TLSPolicyNone:
		return mail.NoTLS
	case integrationkinds.TLSPolicyOpportunistic:
		return mail.TLSOpportunistic
	case integrationkinds.TLSPolicyMandatory:
		return mail.TLSMandatory
	default:
		return mail.TLSMandatory
	}
}
