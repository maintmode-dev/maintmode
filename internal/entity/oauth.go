package entity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

type OAuthProvider string

const (
	OAuthProviderGoogle OAuthProvider = "google"
)

type OAuthState struct {
	// Nonce for CSRF
	Nonce string `json:"nonce"`
	// OriginalURI is used to redirect for post-login navigation.
	OriginalURI string `json:"original_uri,omitempty"`
}

func NewOAuthStateFromB64Json(b64json string) (*OAuthState, error) {
	stateData, err := base64.URLEncoding.DecodeString(b64json)
	if err != nil {
		return nil, fmt.Errorf("b64 decode failed: %w", err)
	}

	state := new(OAuthState)
	if err := json.Unmarshal(stateData, state); err != nil {
		return nil, fmt.Errorf("unmarshal failed: %w", err)
	}

	return state, nil
}

// ToB64Json base64(JSON{...}}
func (s *OAuthState) ToB64Json(ctx context.Context) string {
	stateJSON, err := json.Marshal(s)
	if err != nil {
		xlog.Error(ctx, "failed to encode state", xfield.Error(err))
		stateJSON = []byte("{}")
	}

	return base64.URLEncoding.EncodeToString(stateJSON)
}

type OAuthProviderTokens struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
}

type OAuthProviderUserInfo struct {
	ID    string
	Email string
	Name  string
}
