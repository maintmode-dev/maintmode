package telegram

import (
	"cmp"
	"fmt"
	"time"

	tgbot "github.com/go-telegram/bot"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
)

const defaultTimeout = 10 * time.Second

// Params is the constructor input for the Telegram transport. It is populated
// from the DB-backed integration settings (RUK-196); BotToken is a secret and
// the type intentionally has no Stringer/marshaler.
type Params struct {
	BotToken string
	APIURL   string
	Timeout  time.Duration
}

type Client struct {
	bot *tgbot.Bot
}

// New constructs the transport. The underlying go-telegram library rejects an
// empty or malformed token at construction, so New returns an error in that case
// — an enabled-but-unprovisioned integration fails fast when the transport is
// built rather than silently at first send.
func New(cfg Params) (*Client, error) {
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
