package apiauthmodels

// ExchangeOAuthCodeRequest redeems the one-time code an OAuth dance callback
// placed in the redirect to the frontend.
//
// The code is the only field: an earlier design also carried a nonce, which was
// dropped because it would have traveled in the same redirect URL as the code
// it was meant to protect, and so bought nothing against the one observer that
// binding was for.
type ExchangeOAuthCodeRequest struct {
	Code string `json:"code"`
}
