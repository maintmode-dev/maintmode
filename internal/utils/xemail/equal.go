package xemail

import "strings"

// EqualIgnoreCase reports whether two addresses are the same address, ignoring
// case.
//
// It is the anti-takeover guard of both invitation paths, and deliberately has
// no environment-gated variant: a dev-only bypass that returned true
// unconditionally meant one wrong gate disabled the check entirely. Dev stands
// keep working because the stub provider echoes an email-shaped id_token back
// as the identity.
func EqualIgnoreCase(a, b string) bool {
	return strings.EqualFold(strings.ToLower(a), strings.ToLower(b))
}
