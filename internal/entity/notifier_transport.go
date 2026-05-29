package entity

// NotifyTransport identifies transport in routing config and registries.
type NotifyTransport string

const (
	NotifyTransportStub     NotifyTransport = "stub"
	NotifyTransportSlack    NotifyTransport = "slack"
	NotifyTransportTelegram NotifyTransport = "telegram"
)

func (t NotifyTransport) IsValid() bool {
	switch t {
	case NotifyTransportSlack,
		NotifyTransportTelegram:
		return true
	default:
		return false
	}
}

type NotifyChannel struct {
	ID                 string
	Transport          NotifyTransport
	TransportChannelID string
	Name               string
	Description        string
}
