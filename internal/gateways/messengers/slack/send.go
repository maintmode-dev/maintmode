package slack

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	slackgo "github.com/slack-go/slack"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (c *Client) Send(ctx context.Context, target string, msg entity.Message) error {
	ctx, span := xlog.WithOperationSpan(ctx, "slack.Send")
	defer span.End()

	_, _, err := c.api.PostMessageContext(ctx, target,
		slackgo.MsgOptionText(formatBody(msg), false),
		slackgo.MsgOptionDisableLinkUnfurl(),
	)
	if err != nil {
		xlog.Error(ctx, "slack postMessage", xfield.Error(err))
		return fmt.Errorf("slack postMessage: %w", err)
	}
	return nil
}

func formatBody(msg entity.Message) string {
	if msg.Subject == "" {
		return msg.Body
	}

	return fmt.Sprintf("*%s*"+
		"\n%s",
		msg.Subject,
		msg.Body,
	)
}
