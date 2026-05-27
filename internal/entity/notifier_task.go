package entity

const (
	ProcessorTaskMessagingSend = "messaging.send"
)

// ProcessorTaskPayloadEventNotify is the typed payload stored in each goque task.
type ProcessorTaskPayloadEventNotify struct {
	TransportName MessengerID `json:"transport"`
	Target        string      `json:"target"`
	Subject       string      `json:"subject"`
	Body          string      `json:"body"`
}
