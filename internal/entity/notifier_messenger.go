package entity

// MessengerID identifies a transport in routing config and registries.
type MessengerID string

const (
	MessengerStub     MessengerID = "stub"
	MessengerSlack    MessengerID = "slack"
	MessengerTelegram MessengerID = "telegram"
)

func (t MessengerID) IsValid() bool {
	switch t {
	case MessengerSlack,
		MessengerTelegram:
		return true
	default:
		return false
	}
}

// Message is a rendered notification ready for delivery.
type Message struct {
	Subject string
	Body    string
}
