package telegram

import (
	"cmp"
	"fmt"
	"time"

	tgbot "github.com/go-telegram/bot"

	"github.com/ruko1202/xhttp/client"

	"github.com/ruko1202/maintmode/internal/entity"
)

const defaultTimeout = 10 * time.Second

// Params is the constructor input for the Telegram transport. It is populated
// from the DB-backed integration settings; BotToken is a secret and
// the type intentionally has no Stringer/marshaler.
type Params struct {
	BotToken string
	APIURL   string
	Timeout  time.Duration
	// AllowInternalHosts lifts the dial guard. See
	// config.NotifyTransportConfig.AllowInternalHosts.
	AllowInternalHosts bool
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

	// api_url is typed by an admin, so the client must not become a way to
	// reach this process's neighbors: internal addresses are refused at dial
	// time, and a redirect cannot carry the request -- whose path holds the
	// bot token -- anywhere the admin did not name.
	httpOpts := []client.Option{
		client.WithTimeout(timeout),
		// sanitizer, not xsanitize.New(): the bot token travels in the URL path
		// here, so this gateway needs the Telegram-specific rule on top of the
		// shared header policy.
		client.WithSanitizer(sanitizer{}),
		client.WithoutRedirect(),
	}
	if !cfg.AllowInternalHosts {
		httpOpts = append(httpOpts, client.WithoutInternalHosts())
	}

	opts := []tgbot.Option{
		tgbot.WithHTTPClient(timeout, client.NewClient(httpOpts...)),
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
