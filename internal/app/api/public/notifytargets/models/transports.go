package apimodels

// Transport describes a single supported notification transport for the channel-create UI
type Transport struct {
	ID    string `json:"id" example:"slack"`
	Title string `json:"title" example:"Slack"`
}

// TransportsResponse is the payload of GET /api/v1/notifications/transports.
// Wrapping the slice in an object leaves room for future fields without a
// breaking change.
type TransportsResponse struct {
	Transports []*Transport `json:"transports"`
}
