package entity

import "github.com/golang-jwt/jwt/v5"

type AccessClaims struct {
	Email string `json:"email"`
	Roles []Role `json:"roles"`
	jwt.RegisteredClaims
}

// JWK represents a single JSON Web Key (RFC 7517).
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// JWKS represents a JSON Web Key Set (RFC 7517).
type JWKS struct {
	Keys []JWK `json:"keys"`
}

type IntrospectReport struct {
	Active  bool
	JTI     string
	Subject string
	Email   string
	Roles   []Role
	Exp     int64
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}
