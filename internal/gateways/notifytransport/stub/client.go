package stubtransport

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

type Client struct{}

func New() *Client {
	return &Client{}
}

func (c *Client) TransportID() entity.NotifyTransport {
	return entity.NotifyTransportStub
}

// Send logs the message instead of delivering it.
//
// Threads are not supported by design: nothing is actually delivered, so there
// is no addressable message to reply to. replyTo is logged for debugging and
// the returned SendResult is empty, which means threads never work on a stub
// configuration — expected, not a bug.
func (c *Client) Send(
	ctx context.Context,
	target string,
	msg entity.NotifyMessage,
	replyTo *entity.MessageRef,
) (entity.SendResult, error) {
	_, span := xlog.WithOperationSpan(ctx, "service.Messagins.Stub.Send")
	defer span.End()

	xlog.Info(ctx, "send message to stub",
		xfield.String("target", target),
		xfield.String("message", msg.Body),
		xfield.String("reply_to", lo.FromPtr(replyTo).MessageID),
	)

	return entity.SendResult{}, nil
}
