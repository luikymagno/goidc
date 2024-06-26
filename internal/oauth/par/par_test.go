package par_test

import (
	"errors"
	"testing"

	"github.com/luikymagno/goidc/internal/oauth/par"
	"github.com/luikymagno/goidc/internal/utils"
	"github.com/luikymagno/goidc/pkg/goidc"
	"golang.org/x/crypto/bcrypt"
)

func TestPushAuthorization_ShouldRejectUnauthenticatedClient(t *testing.T) {
	// When
	client := utils.GetTestClient()
	client.AuthnMethod = goidc.ClientSecretPostAuthn

	ctx := utils.GetTestInMemoryContext()
	if err := ctx.CreateOrUpdateClient(client); err != nil {
		panic(err)
	}

	// Then
	_, err := par.PushAuthorization(ctx, utils.PushedAuthorizationRequest{
		ClientAuthnRequest: utils.ClientAuthnRequest{
			ClientID:     client.ID,
			ClientSecret: "invalid_password",
		},
	})

	// Assert
	var jsonError goidc.OAuthBaseError
	if err == nil || !errors.As(err, &jsonError) {
		t.Error("the client should not be authenticated")
		return
	}
	if jsonError.ErrorCode != goidc.InvalidClient {
		t.Errorf("invalid error code: %s", jsonError.ErrorCode)
		return
	}
}

func TestPushAuthorization_ShouldGenerateRequestURI(t *testing.T) {
	// When
	clientSecret := "password"
	hashedClientSecret, _ := bcrypt.GenerateFromPassword([]byte(clientSecret), 0)

	client := utils.GetTestClient()
	client.AuthnMethod = goidc.ClientSecretPostAuthn
	client.HashedSecret = string(hashedClientSecret)

	ctx := utils.GetTestInMemoryContext()
	if err := ctx.CreateOrUpdateClient(client); err != nil {
		panic(err)
	}

	// Then
	requestURI, err := par.PushAuthorization(ctx, utils.PushedAuthorizationRequest{
		ClientAuthnRequest: utils.ClientAuthnRequest{
			ClientID:     utils.TestClientID,
			ClientSecret: clientSecret,
		},
		AuthorizationParameters: goidc.AuthorizationParameters{
			RedirectURI:  client.RedirectURIS[0],
			Scopes:       client.Scopes,
			ResponseType: goidc.CodeResponse,
			ResponseMode: goidc.QueryResponseMode,
		},
	})

	// Assert
	if err != nil {
		t.Errorf("an error happened: %s", err.Error())
		return
	}
	if requestURI == "" {
		t.Error("the request_uri cannot be empty")
		return
	}

	sessions := utils.GetAuthnSessionsFromTestContext(ctx)
	if len(sessions) != 1 {
		t.Error("the should be only one authentication session")
		return
	}

	session := sessions[0]
	if session.RequestURI != requestURI {
		t.Error("the request URI informed is not the same in the session")
		return
	}
}
