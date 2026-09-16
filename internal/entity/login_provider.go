package entity

import "github.com/ruko1202/maintmode/internal/integrationkinds"

// ConfiguredProvider is one stored login provider as a reload needs it:
// identity, settings when they could be opened, and why not when they could
// not.
//
// It lives here, with the rest of the product's vocabulary, rather than in the
// service that produces it or the one that consumes it. That was not possible
// until integrationkinds stopped importing this package -- the two would have
// formed a cycle -- and the workaround was to hand the type to one of the two
// services, which left them trading structs directly. No other pair of services
// in this repository does that: they exchange Service values and interfaces,
// and their data meets here.
type ConfiguredProvider struct {
	// Name is the system, which is also the registry key that decides which
	// implementation parses the row. The category is deliberately absent: every
	// row in such a listing is a login provider by construction, so carrying it
	// would be a constant the caller could only ever re-derive.
	Name string
	// Enabled is the operator's switch. A disabled provider is still reported,
	// because the caller has to stop serving it -- silence would leave the
	// previous snapshot's copy live.
	Enabled bool
	// Settings is nil when the row could not be opened, which Unreadable says.
	Settings integrationkinds.Settings
	// Unreadable means the row could not be opened: its secret would not
	// decrypt, its settings no longer parse, or nothing implements its name.
	//
	// A flag rather than the error itself, decided where the failure happens.
	// The service knows which of those it hit; a caller handed the error could
	// only re-derive it by matching sentinels, and the error is exactly what
	// must not travel -- a decrypt failure's text can carry secret material, so
	// it is logged once, where it arises, with the provider named and nothing
	// else.
	Unreadable bool
}

// LoginProviderHealth reports whether a configured provider can actually be
// used. It is admin-facing only: the sign-in page lists what is configured, not
// what is reachable, and probing every IdP to render a login page would be a
// self-inflicted outage.
//
// It lives here rather than in the service that computes it because it does not
// stop there: the admin API renders it, and the service that installs providers
// sets it. A state the product names in its API is vocabulary, not a service's
// internal enum -- and while it lived in the service, the only way to hand it to
// the API layer without exporting a service type was to flatten it to a plain
// string on the way out.
type LoginProviderHealth string

const (
	// LoginProviderHealthOK means the provider resolved and can sign people in.
	LoginProviderHealthOK LoginProviderHealth = "ok"
	// LoginProviderHealthUnresolved means discovery has not answered, or the row
	// is not in the snapshot at all. The provider is listed and /start refuses
	// it.
	LoginProviderHealthUnresolved LoginProviderHealth = "unresolved"
	// LoginProviderHealthDisabled means an operator turned it off.
	LoginProviderHealthDisabled LoginProviderHealth = "disabled"
	// LoginProviderHealthUnreadable means the stored secret does not decrypt.
	LoginProviderHealthUnreadable LoginProviderHealth = "unreadable"
)

// LoginMethodView is one entry of the sign-in method list: what a caller needs
// to render a button, and nothing else.
//
// It lives here rather than in the service that assembles it because it is read
// by the API layer, which would otherwise have to name a service type to
// describe its own response.
type LoginMethodView struct {
	ID          AuthMethod
	DisplayName string
}
