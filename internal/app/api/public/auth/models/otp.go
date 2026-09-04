package apiauthmodels

// RequestOTPRequest asks for a one-time sign-in code to be emailed.
//
// Email is REQUIRED and must be a well-formed address of at most 254 characters.
// Saying so here matters more than usual: the endpoint answers 202 whatever
// happens — a malformed address included — so a client that gets the field wrong
// receives no runtime signal at all. This contract is the only place that can
// tell an integrator the field is mandatory.
type RequestOTPRequest struct {
	Email string `json:"email"`
}

// RequestOTPResponse carries the session nonce that binds a code to the client
// that asked for it.
//
// It is deliberately not a cookie. The browser never calls this API — the BFF
// does, server-to-server — so a Set-Cookie here would be stored by the BFF's
// HTTP client and never reach the user's browser, leaving a binding that looks
// implemented and binds nothing. The BFF sets the cookie on its own origin,
// where it behaves as intended.
//
// The nonce is returned on every request, including those where no code was
// issued. One returned only for real accounts would answer the very question the
// uniform 202 exists to hide.
type RequestOTPResponse struct {
	SessionNonce string `json:"session_nonce"`
}
