package telegram

import (
	"cmp"
	"fmt"
	"time"

	tgbot "github.com/go-telegram/bot"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
)

const defaultTimeout = 10 * time.Second

type Client struct {
	bot *tgbot.Bot
}

// New constructs the transport. Returns a nil-bot client (Send reports
// it at runtime) when token is empty or invalid — keeps startup
// resilient when env secrets aren't yet provisioned.
func New(cfg config.TelegramConfig) (*Client, error) {
	timeout := cmp.Or(cfg.Timeout, defaultTimeout)

	opts := []tgbot.Option{
		tgbot.WithHTTPClient(timeout, xhttp.NewClient(xhttp.WithTimeout(timeout))),
		tgbot.WithSkipGetMe(), // send-only, no update polling
	}
	if cfg.APIURL != "" {
		opts = append(opts, tgbot.WithServerURL(cfg.APIURL))
	}

	b, err := tgbot.New(cfg.BotToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("telegram bot init failed: %w", err)
	}

	return &Client{bot: b}, nil
}

func (*Client) TransportID() entity.NotifyTransport {
	return entity.NotifyTransportTelegram
}
