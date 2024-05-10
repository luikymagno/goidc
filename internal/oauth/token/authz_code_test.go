package token_test

import (
	"strings"
	"testing"

	"github.com/luikymagno/auth-server/internal/models"
	"github.com/luikymagno/auth-server/internal/oauth/token"
	"github.com/luikymagno/auth-server/internal/unit"
	"github.com/luikymagno/auth-server/internal/unit/constants"
	"github.com/luikymagno/auth-server/internal/utils"
)

func TestAuthorizationCodeHandleGrantCreation(t *testing.T) {

	// When
	ctx, tearDown := utils.SetUpTest()
	defer tearDown()
	client, _ := ctx.ClientManager.Get(models.TestClientId)

	authorizationCode := "random_authz_code"
	session := models.AuthnSession{
		ClientId: models.TestClientId,
		AuthorizationParameters: models.AuthorizationParameters{
			Scope:       strings.Join(client.Scopes, " "),
			RedirectUri: client.RedirectUris[0],
		},
		AuthorizationCode:     authorizationCode,
		Subject:               "user_id",
		CreatedAtTimestamp:    unit.GetTimestampNow(),
		AuthorizedAtTimestamp: unit.GetTimestampNow(),
		Store:                 make(map[string]string),
		AdditionalTokenClaims: make(map[string]string),
	}
	ctx.AuthnSessionManager.CreateOrUpdate(session)

	req := models.TokenRequest{
		ClientAuthnRequest: models.ClientAuthnRequest{
			ClientIdPost:     models.TestClientId,
			ClientSecretPost: models.TestClientSecret,
		},
		GrantType:         constants.AuthorizationCodeGrant,
		RedirectUri:       client.RedirectUris[0],
		AuthorizationCode: authorizationCode,
	}

	// Then
	token, err := token.HandleGrantCreation(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("no error should be returned: %s", err.Error())
		return
	}

	if token.ClientId != models.TestClientId {
		t.Error("the token was assigned to a different client")
		return
	}

	if token.Subject != session.Subject {
		t.Error("the token subject should be the client")
		return
	}

	grantSessions := utils.GetGrantSessionsFromTestContext(ctx)
	if len(grantSessions) != 1 {
		t.Error("there should be only one grant session")
		return
	}
}
