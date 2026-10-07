package apiauthmodels

// ExchangeOAuthCodeRequest redeems the one-time code an OAuth dance callback
// placed in the redirect to the frontend.
//
// BindingProof is the nonce whose hash /start received as binding. It never
// travels through the redirect: the BFF keeps it in a cookie of its own, so a
// code carried to another browser arrives without it.
type ExchangeOAuthCodeRequest struct {
	Code         string `json:"code"`
	BindingProof string `json:"binding_proof"`
}

// CompleteLinkRequest redeems the link_code a link-mode callback put in the
// redirect, with the same binding proof as a sign-in code.
type CompleteLinkRequest struct {
	LinkCode     string `json:"link_code"`
	BindingProof string `json:"binding_proof"`
}
