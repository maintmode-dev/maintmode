package stubtransport

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
	return entity.NotifyTransportStub
}

func (c *Client) Send(ctx context.Context, target string, msg entity.NotifyMessage) error {
	_, span := xlog.WithOperationSpan(ctx, "service.Messagins.Stub.Send")
	defer span.End()

	xlog.Info(ctx, "send message to stub",
		xfield.String("target", target),
		xfield.String("message", msg.Body),
	)

	return nil
}
