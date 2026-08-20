package entity

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AccessClaims struct {
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
	UserRoles []Role `json:"user_roles"`
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
	// SessionID is the family of the issued refresh token: a stable session
	// identifier that survives rotation. Populated on the initial login
	// (IssueTokenPair) and used by the audit trail; never returned in the API response.
	SessionID uuid.UUID
}
