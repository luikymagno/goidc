package utils

import (
	"log/slog"
	"strings"

	"github.com/luikymagno/auth-server/internal/issues"
	"github.com/luikymagno/auth-server/internal/models"
	"github.com/luikymagno/auth-server/internal/unit"
	"github.com/luikymagno/auth-server/internal/unit/constants"
)

func HandleTokenCreation(
	ctx Context,
	req models.TokenRequest,
) (models.Token, error) {

	var token models.Token
	var err error
	switch req.GrantType {
	case constants.ClientCredentials:
		token, err = handleClientCredentialsGrantTokenCreation(ctx, req)
	case constants.AuthorizationCode:
		token, err = handleAuthorizationCodeGrantTokenCreation(ctx, req)
	default:
		err = issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid grant type",
		}
	}
	if err != nil {
		return models.Token{}, err
	}

	err = ctx.CrudManager.TokenSessionManager.Create(token)
	if err != nil {
		return models.Token{}, err
	}

	return token, nil
}

//---------------------------------------- Client Credentials ----------------------------------------//

func handleClientCredentialsGrantTokenCreation(ctx Context, req models.TokenRequest) (models.Token, error) {
	authenticatedClient, err := getAuthenticatedClient(ctx, req.ClientAuthnRequest)
	if err != nil {
		return models.Token{}, err
	}

	if err := validateClientCredentialsGrantRequest(ctx, authenticatedClient, req); err != nil {
		return models.Token{}, err
	}

	tokenModel, err := ctx.CrudManager.TokenModelManager.Get(authenticatedClient.DefaultTokenModelId)
	if err != nil {
		return models.Token{}, err
	}

	return tokenModel.GenerateToken(models.TokenContextInfo{
		Subject:  authenticatedClient.Id,
		ClientId: authenticatedClient.Id,
		Scopes:   unit.SplitString(req.Scope),
	}), nil
}

func validateClientCredentialsGrantRequest(ctx Context, client models.Client, req models.TokenRequest) error {
	if req.RedirectUri != "" || req.AuthorizationCode != "" {
		ctx.Logger.Info("invalid parameter for client credentials")
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid parameter for client credentials",
		}
	}

	if !client.IsGrantTypeAllowed(constants.ClientCredentials) {
		ctx.Logger.Info("grant type not allowed")
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid grant type",
		}
	}

	scopes := []string{}
	if req.Scope != "" {
		scopes = strings.Split(req.Scope, " ")
	}
	if !client.AreScopesAllowed(scopes) {
		ctx.Logger.Info("scope not allowed")
		return issues.JsonError{
			ErrorCode:        constants.InvalidScope,
			ErrorDescription: "invalid scope",
		}
	}

	return nil
}

//---------------------------------------- Authorization Code ----------------------------------------//

func handleAuthorizationCodeGrantTokenCreation(ctx Context, req models.TokenRequest) (models.Token, error) {
	if req.AuthorizationCode == "" {
		return models.Token{}, issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid authorization code",
		}
	}

	authenticatedClient, session, err := getAuthenticatedClientAndSession(ctx, req)
	if err != nil {
		ctx.Logger.Debug("error while loading the client or session.", slog.String("error", err.Error()))
		return models.Token{}, err
	}

	if err := validateAuthorizationCodeGrantRequest(req, authenticatedClient, session); err != nil {
		ctx.Logger.Debug("invalid parameters for the token request.", slog.String("error", err.Error()))
		return models.Token{}, err
	}

	ctx.Logger.Debug("get the token model.")
	tokenModel, err := ctx.CrudManager.TokenModelManager.Get(authenticatedClient.DefaultTokenModelId)
	if err != nil {
		ctx.Logger.Debug("error while loading the token model.", slog.String("error", err.Error()))
		return models.Token{}, err
	}
	ctx.Logger.Debug("the token model was loaded successfully.")

	return tokenModel.GenerateToken(models.TokenContextInfo{
		Subject:  session.Subject,
		ClientId: session.ClientId,
		Scopes:   session.Scopes,
	}), nil
}

func validateAuthorizationCodeGrantRequest(req models.TokenRequest, client models.Client, session models.AuthnSession) error {
	if req.Scope != "" || req.AuthorizationCode == "" || req.RedirectUri == "" {
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid parameter for authorization code",
		}
	}

	if !client.IsGrantTypeAllowed(constants.AuthorizationCode) {
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "invalid grant type",
		}
	}

	if session.AuthorizedAtTimestamp+constants.AuthorizationCodeLifetimeSecs > unit.GetTimestampNow() {
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "the authorization code is expired",
		}
	}

	if session.ClientId != req.ClientId {
		return issues.JsonError{
			ErrorCode:        constants.InvalidRequest,
			ErrorDescription: "the authorization code was not issued to the client",
		}
	}

	return nil
}

func getAuthenticatedClientAndSession(ctx Context, req models.TokenRequest) (models.Client, models.AuthnSession, error) {
	ctx.Logger.Debug("get the session using the authorization code.")
	type sessionResultType struct {
		session models.AuthnSession
		err     error
	}
	sessionCh := make(chan sessionResultType)
	go func(chan<- sessionResultType) {
		session, err := ctx.CrudManager.AuthnSessionManager.GetByAuthorizationCode(req.AuthorizationCode)
		sessionCh <- sessionResultType{
			session: session,
			err:     err,
		}
		// Always delete the session.
		ctx.CrudManager.AuthnSessionManager.Delete(session.Id)
	}(sessionCh)

	// Fetch the client while the session is being fetched.
	ctx.Logger.Debug("get the client while the session is being loaded.")
	authenticatedClient, err := getAuthenticatedClient(ctx, req.ClientAuthnRequest)
	if err != nil {
		ctx.Logger.Debug("error while loading the client.", slog.String("error", err.Error()))
		return models.Client{}, models.AuthnSession{}, err
	}
	ctx.Logger.Debug("the client was loaded successfully.")

	ctx.Logger.Debug("wait for the session.")
	sessionResult := <-sessionCh
	session, err := sessionResult.session, sessionResult.err
	if err != nil {
		ctx.Logger.Debug("error while loading the session.", slog.String("error", err.Error()))
		return models.Client{}, models.AuthnSession{}, err
	}
	ctx.Logger.Debug("the session was loaded successfully.")

	return authenticatedClient, session, nil
}

func getAuthenticatedClient(ctx Context, req models.ClientAuthnRequest) (models.Client, error) {
	// Fetch the client.
	client, err := ctx.CrudManager.ClientManager.Get(req.ClientId)
	if err != nil {
		ctx.Logger.Info("client not found", slog.String("client_id", req.ClientId))
		return models.Client{}, err
	}

	// Verify that the client is authenticated.
	if !client.Authenticator.IsAuthenticated(req) {
		ctx.Logger.Info("client not authenticated", slog.String("client_id", req.ClientId))
		return models.Client{}, issues.JsonError{
			ErrorCode:        constants.AccessDenied,
			ErrorDescription: "client not authenticated",
		}
	}

	return client, nil
}
