package utils

import (
	"strings"

	"github.com/luikymagno/auth-server/internal/crud"
	"github.com/luikymagno/auth-server/internal/issues"
	"github.com/luikymagno/auth-server/internal/models"
	"github.com/luikymagno/auth-server/internal/unit/constants"
)

func HandleTokenCreation(
	ctx Context,
	request models.TokenRequest,
) (models.Token, error) {

	grantInfo, err := getGrantInfo(ctx, request)
	if err != nil {
		return models.Token{}, err
	}

	var token models.Token
	switch grantInfo.GrantType {
	case constants.ClientCredentials:
		token, err = handleClientCredentialsGrantTokenCreation(grantInfo)
	case constants.AuthorizationCode:
		token, err = handleAuthorizationCodeGrantTokenCreation(ctx, grantInfo)
	default:
		err = issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid grant type",
		}
	}
	if err != nil {
		return models.Token{}, err
	}

	errorCh := make(chan error, 1)
	ctx.CrudManager.TokenSessionManager.Create(token, errorCh)
	if err = <-errorCh; err != nil {
		return models.Token{}, err
	}

	return token, nil
}

func getGrantInfo(
	ctx Context,
	request models.TokenRequest,
) (
	models.GrantInfo,
	error,
) {
	authenticatedClient, err := getAuthenticatedClient(
		ctx.CrudManager.ClientManager,
		models.ClientAuthnContext{
			ClientId:     request.ClientId,
			ClientSecret: request.ClientSecret,
		},
	)
	if err != nil {
		return models.GrantInfo{}, err
	}

	tokenModelCh := make(chan crud.TokenModelGetResult, 1)
	ctx.CrudManager.TokenModelManager.Get(authenticatedClient.DefaultTokenModelId, tokenModelCh)
	tokenModelResult := <-tokenModelCh
	if tokenModelResult.Error != nil {
		return models.GrantInfo{}, tokenModelResult.Error
	}

	scopes := []string{}
	if request.Scope != "" {
		scopes = strings.Split(request.Scope, " ")
	}
	return models.GrantInfo{
		GrantType:           request.GrantType,
		AuthenticatedClient: authenticatedClient,
		TokenModel:          tokenModelResult.TokenModel,
		Scopes:              scopes,
		AuthorizationCode:   request.AuthorizationCode,
		RedirectUri:         request.RedirectUri,
	}, nil
}

//---------------------------------------- Client Credentials ----------------------------------------//

func handleClientCredentialsGrantTokenCreation(grantInfo models.GrantInfo) (models.Token, error) {
	if err := validateClientCredentialsGrantRequest(grantInfo); err != nil {
		return models.Token{}, err
	}

	return grantInfo.TokenModel.GenerateToken(models.TokenContextInfo{
		Subject:  grantInfo.AuthenticatedClient.Id,
		ClientId: grantInfo.AuthenticatedClient.Id,
		Scopes:   grantInfo.Scopes,
	}), nil
}

func validateClientCredentialsGrantRequest(grantInfo models.GrantInfo) error {
	if !grantInfo.AuthenticatedClient.IsGrantTypeAllowed(constants.ClientCredentials) {
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid grant type",
		}
	}
	if grantInfo.RedirectUri != "" || grantInfo.AuthorizationCode != "" {
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid parameter for client credentials",
		}
	}
	if !grantInfo.AuthenticatedClient.AreScopesAllowed(grantInfo.Scopes) {
		return issues.JsonError{
			ErrorCode:        constants.InvalidScope,
			ErrorDescription: "invalid scope",
		}
	}

	return nil
}

//---------------------------------------- Authorization Code ----------------------------------------//

func handleAuthorizationCodeGrantTokenCreation(ctx Context, grantInfo models.GrantInfo) (models.Token, error) {
	sessionCh := make(chan crud.AuthnSessionGetResult, 1)
	ctx.CrudManager.AuthnSessionManager.GetByAuthorizationCode(grantInfo.AuthorizationCode, sessionCh)
	sessionResult := <-sessionCh
	if sessionResult.Error != nil {
		return models.Token{}, issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid authorization code",
		}
	}
	// Always delete the session at the end.
	go ctx.CrudManager.AuthnSessionManager.Delete(sessionResult.Session.Id)

	if err := validateAuthorizationCodeGrantRequest(grantInfo, sessionResult.Session); err != nil {
		return models.Token{}, err
	}

	return grantInfo.TokenModel.GenerateToken(models.TokenContextInfo{
		Subject:  sessionResult.Session.Subject,
		ClientId: sessionResult.Session.ClientId,
		Scopes:   sessionResult.Session.Scopes,
	}), nil
}

func validateAuthorizationCodeGrantRequest(grantInfo models.GrantInfo, session models.AuthnSession) error {
	if !grantInfo.AuthenticatedClient.IsGrantTypeAllowed(constants.AuthorizationCode) {
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid grant type",
		}
	}
	if len(grantInfo.Scopes) != 0 || grantInfo.AuthorizationCode == "" || grantInfo.RedirectUri == "" {
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid parameter for authorization code",
		}
	}
	if session.ClientId != grantInfo.AuthenticatedClient.Id {
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "the authorization code was not issued to the client",
		}
	}

	return nil
}

func getAuthenticatedClient(clientManager crud.ClientManager, authnContext models.ClientAuthnContext) (models.Client, error) {
	clientCh := make(chan crud.ClientGetResult, 1)
	clientManager.Get(authnContext.ClientId, clientCh)
	clientResult := <-clientCh
	if clientResult.Error != nil {
		return models.Client{}, clientResult.Error
	}

	clientAuthnContext := models.ClientAuthnContext{
		ClientSecret: authnContext.ClientSecret,
	}
	if !clientResult.Client.Authenticator.IsAuthenticated(clientAuthnContext) {
		return models.Client{}, issues.JsonError{
			ErrorCode:        constants.AccessDenied,
			ErrorDescription: "client not authenticated",
		}
	}

	return clientResult.Client, nil
}
