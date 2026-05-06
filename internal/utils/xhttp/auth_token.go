package xhttp

import (
	"net/http"
	"strings"
)

const bearerHeader = "Bearer "

// ExtractBearerToken returns the raw Bearer token from the Authorization header,
// or empty string if absent/malformed.
func ExtractBearerToken(req *http.Request) string {
	authHeader := req.Header.Get("Authorization")

	token, ok := strings.CutPrefix(authHeader, bearerHeader)
	if !ok {
		return ""
	}

	return token
}

func SetBearerToken(req *http.Request, token string) {
	req.Header.Set("Authorization", bearerHeader+token)
}
