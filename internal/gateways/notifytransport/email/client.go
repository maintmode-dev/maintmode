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
	"fmt"
	"time"

	"github.com/wneessen/go-mail"

	"github.com/ruko1202/maintmode/internal/entity"
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

func (*Client) TransportID() entity.NotifyTransport {
	return entity.NotifyTransportEmail
}

// TLS policy config strings. Unknown/empty values default to mandatory STARTTLS —
// the safe production posture. tlsPolicyNone is the plaintext mode used by the
// in-process SMTP test server.
const (
	tlsPolicyNone          = "none"
	tlsPolicyOpportunistic = "opportunistic"
	tlsPolicyTLSMandatory  = "mandatory"
)

// tlsPolicy maps the config string to a go-mail TLSPolicy.
func tlsPolicy(s string) mail.TLSPolicy {
	switch s {
	case tlsPolicyNone:
		return mail.NoTLS
	case tlsPolicyOpportunistic:
		return mail.TLSOpportunistic
	case tlsPolicyTLSMandatory:
		return mail.TLSMandatory
	default:
		return mail.TLSMandatory
	}
}
