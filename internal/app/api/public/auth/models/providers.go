package apiauthmodels

// AuthMethodType tells the login page how to render a method. It is the only
// thing that varies the page's shape, which is why it is a field rather than
// something the client infers from the id.
type AuthMethodType string

const (
	// AuthMethodTypePassword renders a form posting to /login/password.
	AuthMethodTypePassword AuthMethodType = "password"
	// AuthMethodTypeCode renders the two-step emailed-code flow.
	AuthMethodTypeCode AuthMethodType = "code"
	// AuthMethodTypeRedirect renders a button leading to an external provider.
	// Reserved: no element carries it today, and external providers still go
	// through the BFF rather than appearing here.
	AuthMethodTypeRedirect AuthMethodType = "redirect"
)

// AuthMethodsResponse is the public list of ways to sign in.
//
// It is deliberately the whole contract. Nothing here identifies a provider's
// client id, issuer or any URL, and nothing varies with the caller — two
// requests from an anonymous visitor and from a known user's browser return
// byte-identical bodies. An endpoint that answered differently for a known
// address would be a user-enumeration oracle wearing a configuration endpoint's
// clothes.
type AuthMethodsResponse struct {
	Methods []AuthMethod `json:"methods"`
}

// AuthMethod is one way to sign in: what to call it, and how to draw it.
type AuthMethod struct {
	ID          string         `json:"id"`
	Type        AuthMethodType `json:"type"`
	DisplayName string         `json:"display_name"`
}
