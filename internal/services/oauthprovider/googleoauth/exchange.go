package googleoauth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"golang.org/x/oauth2"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Exchange trades an authorization code for Google tokens.
func (g *Service) Exchange(ctx context.Context, code string) (*entity.OAuthProviderTokens, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.OAuth.Google.Exchange")
	defer span.End()

	ctx = context.WithValue(ctx, oauth2.HTTPClient, g.httpClient)

	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		xlog.Error(ctx, "failed to exchange google token", xfield.Error(err))
		return nil, fmt.Errorf("google token exchange: %w", err)
	}

	result := &entity.OAuthProviderTokens{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
	}

	if idToken, ok := token.Extra("id_token").(string); ok {
		result.IDToken = idToken
	}

	return result, nil
}
