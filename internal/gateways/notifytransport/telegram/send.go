package telegram

import (
	"context"
	"fmt"
	"strconv"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Send delivers msg to target (numeric chat_id or @channelname), replying to
// replyTo when one is given.
//
// Telegram groups have no real threads; a chain of replies to the root message
// is the closest equivalent and is a deliberate platform limitation, not a gap.
func (c *Client) Send(
	ctx context.Context,
	target string,
	msg entity.NotifyMessage,
	replyTo *entity.MessageRef,
) (entity.SendResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "telegram.Send")
	defer span.End()

	replyParams := c.replyParams(ctx, replyTo)

	sent, err := c.bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          target,
		Text:            formatBody(msg),
		ReplyParameters: replyParams,
	})
	if err != nil {
		xlog.Error(ctx, "telegram sendMessage", xfield.Error(err))
		return entity.SendResult{}, fmt.Errorf("telegram sendMessage: %w", err)
	}

	// AllowSendingWithoutReply makes the server deliver a plain message instead
	// of failing when the root is gone, and it signals that by omitting
	// reply_to_message. Logged so that "everything has been going out flat for a
	// month" is visible rather than silent.
	//
	// RootReplaced stays false: a dropped reply is not proof the root is dead
	// forever, the degradation costs nothing, and clearing the root here would
	// throw away a thread that may well still work on the next event.
	if replyParams != nil && sent.ReplyToMessage == nil {
		xlog.Warn(ctx, "telegram ignored the reply, message delivered flat",
			xfield.String("root_message_id", replyTo.MessageID))
	}

	// The numeric chat the message landed in is logged rather than stored:
	// nothing reads it back, but it is what tells an operator which chat a
	// thread actually went to when one surfaces somewhere unexpected.
	xlog.Debug(ctx, "telegram message delivered",
		xfield.Int64("chat_id", sent.Chat.ID),
		xfield.Int("message_id", sent.ID))

	return entity.SendResult{Ref: newRef(sent)}, nil
}

// replyParams converts the domain reference into Telegram's request shape.
//
// MessageRef.MessageID is a string across the domain (Slack ts is not numeric),
// while Telegram wants an int, so the conversion happens here — inside the one
// client that needs it — rather than leaking a numeric id into entity or the
// database. Garbage in the column degrades the message to flat instead of
// dropping it: delivery outranks threading.
func (c *Client) replyParams(ctx context.Context, replyTo *entity.MessageRef) *models.ReplyParameters {
	if replyTo == nil {
		return nil
	}

	id, err := strconv.Atoi(replyTo.MessageID)
	if err != nil {
		xlog.Warn(ctx, "telegram: unparsable root message id, sending flat",
			xfield.String("root_message_id", replyTo.MessageID),
			xfield.Error(err))

		return nil
	}

	return &models.ReplyParameters{
		MessageID:                id,
		AllowSendingWithoutReply: true,
	}
}

// newRef carries the id of the message just sent. It is stringified because the
// domain keeps message ids as strings, so a Slack ts never travels through a
// numeric type.
func newRef(sent *models.Message) *entity.MessageRef {
	if sent == nil {
		return nil
	}

	return &entity.MessageRef{MessageID: strconv.Itoa(sent.ID)}
}

func formatBody(msg entity.NotifyMessage) string {
	if msg.Subject == "" {
		return msg.Body
	}

	return fmt.Sprintf("%s"+
		"\n\n%s",
		msg.Subject,
		msg.Body,
	)
}
