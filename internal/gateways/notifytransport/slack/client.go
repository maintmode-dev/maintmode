package slack

import (
	"cmp"
	"time"

	slackgo "github.com/slack-go/slack"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
)

const defaultTimeout = 10 * time.Second

// Params is the constructor input for the Slack transport. It is populated from
// the DB-backed integration settings (RUK-196); BotToken is a secret and the
// type intentionally has no Stringer/marshaler.
type Params struct {
	BotToken string
	APIURL   string
	Timeout  time.Duration
}

type Client struct {
	api *slackgo.Client // nil if bot_token absent
}

// New constructs the transport. Returns a nil-api client (Send reports
// it at runtime) when bot_token is empty — keeps startup resilient.
func New(cfg Params) *Client {
	opts := []slackgo.Option{
		slackgo.OptionHTTPClient(xhttp.NewClient(xhttp.WithTimeout(cmp.Or(cfg.Timeout, defaultTimeout)))),
	}
	if cfg.APIURL != "" {
		opts = append(opts, slackgo.OptionAPIURL(cfg.APIURL))
	}

	return &Client{api: slackgo.New(cfg.BotToken, opts...)}
}

func (*Client) TransportID() entity.NotifyTransport {
	return entity.NotifyTransportSlack
}
