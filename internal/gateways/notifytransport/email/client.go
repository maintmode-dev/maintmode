// Package emailtransport is the email notify transport.
//
// Real delivery (SMTP/SES/SendGrid) is a separate infra task (RUK-94 open
// question #4). Until it lands, Send logs the message so an invitation link can
// be copied to the recipient manually. Wiring email as a transport keeps it in
// the same registry as slack/telegram rather than a parallel abstraction.
package emailtransport

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

type Client struct{}

func New() *Client {
	return &Client{}
}

func (c *Client) TransportID() entity.NotifyTransport {
	return entity.NotifyTransportEmail
}

func (c *Client) Send(ctx context.Context, target string, msg entity.NotifyMessage) error {
	ctx, span := xlog.WithOperationSpan(ctx, "transport.Email.Send")
	defer span.End()

	// Stub delivery: log so the recipient address, subject and body (which
	// carries the invitation link) can be copied manually until real email
	// delivery is configured.
	xlog.Info(ctx, "email transport (stub delivery — copy manually)",
		xfield.String("target", target),
		xfield.String("subject", msg.Subject),
		xfield.String("body", msg.Body),
	)

	return nil
}
