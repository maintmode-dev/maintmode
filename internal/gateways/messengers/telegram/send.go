package telegram

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Send delivers msg to target (numeric chat_id or @channelname).
func (c *Client) Send(ctx context.Context, target string, msg entity.Message) error {
	ctx, span := xlog.WithOperationSpan(ctx, "telegram.Send")
	defer span.End()

	_, err := c.bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: target,
		Text:   formatBody(msg),
	})
	if err != nil {
		xlog.Error(ctx, "telegram sendMessage", xfield.Error(err))
		return fmt.Errorf("telegram sendMessage: %w", err)
	}
	return nil
}

func formatBody(msg entity.Message) string {
	if msg.Subject == "" {
		return msg.Body
	}

	return fmt.Sprintf("%s"+
		"\n\n%s",
		msg.Subject,
		msg.Body,
	)
}
