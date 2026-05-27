// Package slack implements the messengers.Messenger for Slack via chat.postMessage.
package slack

import (
	"cmp"
	"time"

	slackgo "github.com/slack-go/slack"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
)

const defaultTimeout = 10 * time.Second

type Client struct {
	api *slackgo.Client // nil if bot_token absent
}

// New constructs the transport. Returns a nil-api client (Send reports
// it at runtime) when bot_token is empty — keeps startup resilient.
func New(cfg config.SlackConfig) *Client {
	opts := []slackgo.Option{
		slackgo.OptionHTTPClient(xhttp.NewClient(xhttp.WithTimeout(cmp.Or(cfg.Timeout, defaultTimeout)))),
	}
	if cfg.APIURL != "" {
		opts = append(opts, slackgo.OptionAPIURL(cfg.APIURL))
	}

	return &Client{api: slackgo.New(cfg.BotToken, opts...)}
}

func (*Client) MessengerID() entity.MessengerID {
	return entity.MessengerSlack
}
