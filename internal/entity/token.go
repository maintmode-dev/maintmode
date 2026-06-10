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
	// SessionID — family выпущенного refresh-токена: стабильный идентификатор
	// сессии, переживающий ротацию. Заполняется при первичном логине
	// (IssueTokenPair) и используется аудитом; в API-ответ не попадает.
	SessionID uuid.UUID
}
