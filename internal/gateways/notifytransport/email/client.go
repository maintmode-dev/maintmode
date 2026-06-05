// Package emailtransport delivers notifications over SMTP. It implements the
// notify Transport (TransportID + Send) so email lives in the same registry as
// slack/telegram rather than a parallel abstraction.
//
// Real delivery is opt-in via config (EmailConfig.Enabled). New fails fast when
// email is enabled but misconfigured (e.g. an empty host from a missing secret),
// so the binary refuses to start rather than silently never delivering. In dev
// the registry's UseStub fallback short-circuits to the stub transport, and the
// email transport is simply left out of the registry when disabled.
package emailtransport

import (
	"cmp"
	"fmt"
	"time"

	"github.com/wneessen/go-mail"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

const (
	defaultTimeout = 10 * time.Second
	defaultPort    = 587
)

type Client struct {
	client  *mail.Client
	from    string
	replyTo string
}

// New constructs the transport from cfg. It returns an error when the transport
// is misconfigured — an empty Host or From — so an enabled-but-misconfigured
// email transport fails at startup instead of silently at first send.
func New(cfg config.EmailConfig) (*Client, error) {
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

// tlsPolicy maps the config string to a go-mail TLSPolicy. Unknown/empty values
// default to mandatory STARTTLS — the safe production posture. "none" is the
// plaintext mode used by the in-process SMTP test server.
func tlsPolicy(s string) mail.TLSPolicy {
	switch s {
	case "none":
		return mail.NoTLS
	case "opportunistic":
		return mail.TLSOpportunistic
	default:
		return mail.TLSMandatory
	}
}
