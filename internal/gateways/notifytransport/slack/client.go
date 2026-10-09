package slack

import (
	"cmp"
	"time"

	slackgo "github.com/slack-go/slack"

	"github.com/ruko1202/xhttp/client"

	"github.com/ruko1202/maintmode/internal/utils/xsanitize"

	"github.com/ruko1202/maintmode/internal/entity"
)

const defaultTimeout = 10 * time.Second

// Params is the constructor input for the Slack transport. It is populated from
// the DB-backed integration settings; BotToken is a secret and the
// type intentionally has no Stringer/marshaler.
type Params struct {
	BotToken string
	APIURL   string
	Timeout  time.Duration
	// AllowInternalHosts lifts the dial guard. See
	// config.NotifyTransportConfig.AllowInternalHosts.
	AllowInternalHosts bool
}

type Client struct {
	api *slackgo.Client // nil if bot_token absent
}

// New constructs the transport. Returns a nil-api client (Send reports
// it at runtime) when bot_token is empty — keeps startup resilient.
func New(cfg Params) *Client {
	// api_url is typed by an admin, so the client must not become a way to
	// reach this process's neighbors: internal addresses are refused at dial
	// time, and a redirect cannot carry the request (bot token included)
	// anywhere the admin did not name.
	httpOpts := []client.Option{
		client.WithTimeout(cmp.Or(cfg.Timeout, defaultTimeout)),
		client.WithSanitizer(xsanitize.New()),
		client.WithoutRedirect(),
	}
	if !cfg.AllowInternalHosts {
		httpOpts = append(httpOpts, client.WithoutInternalHosts())
	}

	opts := []slackgo.Option{
		slackgo.OptionHTTPClient(bearerClient{next: client.NewClient(httpOpts...), token: cfg.BotToken}),
	}
	if cfg.APIURL != "" {
		opts = append(opts, slackgo.OptionAPIURL(cfg.APIURL))
	}

	return &Client{api: slackgo.New(cfg.BotToken, opts...)}
}

func (*Client) TransportID() entity.NotifyTransport {
	return entity.NotifyTransportSlack
}
