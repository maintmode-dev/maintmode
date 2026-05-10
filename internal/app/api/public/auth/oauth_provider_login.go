package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/config"
)

// GoogleOAuthLogin godoc
// @Summary Start OAuth login
// @Description Redirects user to OAuth provider consent page. Optional original_uri preserves frontend path.
// @Tags Auth
// @Produce plain
// @Param original_uri query string false "Original frontend path to redirect after login"
// @Success 307 "Temporary redirect to OAuth provider"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/login/oauth/google [get]
// GoogleOAuthLogin redirects the user to oauth-provider's OAuth consent screen.
// Accepts optional ?original_uri=/path to preserve navigation intent through the OAuth flow.
func (i *Implementation) GoogleOAuthLogin(c *echo.Context) error {
	return i.oauthLogin(c, entity.OAuthProviderGoogle)
}

func (i *Implementation) oauthLogin(c *echo.Context, provider entity.OAuthProvider) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), fmt.Sprintf("api.Auth.%s.Login", provider))
	defer span.End()
	op := "oauth provider login"

	nonce := generateNonce(ctx)
	setCookieNonce(c, nonce)

	encodedState, err := i.stateCodec.Encode(ctx, &entity.OAuthState{
		Nonce:       nonce,
		OriginalURI: c.QueryParam("original_uri"),
	})
	if err != nil {
		xlog.Error(ctx, "failed to encode oauth state", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	url, err := i.authSrv.GetAuthCodeURL(ctx, &entity.GetAuthCodeURLCmd{
		Provider: provider,
		State:    encodedState,
	})
	if err != nil || url == "" {
		err := fmt.Errorf("%w: auth code url is empty", err)

		xlog.Error(ctx, op, xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func generateNonce(ctx context.Context) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		xlog.Error(ctx, "failed to generate random bytes", xfield.Error(err))
		b = xuuid.NewBytes()
	}

	return base64.URLEncoding.EncodeToString(b)
}

var (
	cookieNonceName   = fmt.Sprintf("%s-nonce", config.GetAuthAppBuildMeta().AppName)
	cookieNoncePath   = "/"
	cookieNonceMaxAge = 300 // 5 minutes
)

func setCookieNonce(c *echo.Context, stateNonce string) {
	c.SetCookie(&http.Cookie{
		Name:     cookieNonceName,
		Value:    stateNonce,
		Path:     cookieNoncePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   cookieNonceMaxAge, // 5 minutes
	})
}

func clearCookieNonce(c *echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     cookieNonceName,
		Value:    "",
		Path:     cookieNoncePath,
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	})
}
