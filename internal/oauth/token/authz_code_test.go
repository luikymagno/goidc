package token_test

import (
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/luikymagno/goidc/internal/oauth/token"
	"github.com/luikymagno/goidc/internal/utils"
	"github.com/luikymagno/goidc/pkg/goidc"
)

func TestHandleGrantCreation_AuthorizationCodeGrantHappyPath(t *testing.T) {

	// When
	ctx := utils.GetTestContext(t)

	authorizationCode := "random_authz_code"
	session := goidc.AuthnSession{
		ClientID:      utils.TestClientID,
		GrantedScopes: goidc.ScopeOpenID.String(),
		AuthorizationParameters: goidc.AuthorizationParameters{
			Scopes:      goidc.ScopeOpenID.String(),
			RedirectURI: utils.TestClientRedirectURI,
		},
		AuthorizationCode:         authorizationCode,
		Subject:                   "user_id",
		CreatedAtTimestamp:        goidc.TimestampNow(),
		AuthorizationCodeIssuedAt: goidc.TimestampNow(),
		ExpiresAtTimestamp:        goidc.TimestampNow() + 60,
		Store:                     make(map[string]any),
		AdditionalTokenClaims:     make(map[string]any),
	}
	if err := ctx.CreateOrUpdateAuthnSession(session); err != nil {
		panic(err)
	}

	req := utils.TokenRequest{
		ClientAuthnRequest: utils.ClientAuthnRequest{
			ClientID: utils.TestClientID,
		},
		GrantType:         goidc.GrantAuthorizationCode,
		RedirectURI:       utils.TestClientRedirectURI,
		AuthorizationCode: authorizationCode,
	}

	// Then
	tokenResp, err := token.HandleTokenCreation(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("no error should be returned: %s", err.Error())
		return
	}

	parsedToken, err := jwt.ParseSigned(tokenResp.AccessToken, []jose.SignatureAlgorithm{jose.PS256, jose.RS256})
	if err != nil {
		t.Error("invalid token")
		return
	}

	var claims map[string]any
	err = parsedToken.UnsafeClaimsWithoutVerification(&claims)
	if err != nil {
		t.Error("could not read claims")
		return
	}

	if claims["client_id"].(string) != utils.TestClientID {
		t.Error("the token was assigned to a different client")
		return
	}

	if claims["sub"].(string) != session.Subject {
		t.Error("the token subject should be the client")
		return
	}

	grantSessions := utils.GetGrantSessionsFromTestContext(t, ctx)
	if len(grantSessions) != 1 {
		t.Error("there should be only one grant session")
		return
	}
}
