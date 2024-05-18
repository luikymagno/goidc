package token

import (
	"log/slog"
	"slices"

	"github.com/luikymagno/auth-server/internal/issues"
	"github.com/luikymagno/auth-server/internal/models"
	"github.com/luikymagno/auth-server/internal/unit"
	"github.com/luikymagno/auth-server/internal/unit/constants"
	"github.com/luikymagno/auth-server/internal/utils"
)

func handleAuthorizationCodeGrantTokenCreation(ctx utils.Context, req models.TokenRequest) (models.GrantSession, issues.OAuthError) {

	if oauthErr := preValidateAuthorizationCodeGrantRequest(req); oauthErr != nil {
		return models.GrantSession{}, oauthErr
	}

	authenticatedClient, session, oauthErr := getAuthenticatedClientAndSession(ctx, req)
	if oauthErr != nil {
		ctx.Logger.Debug("error while loading the client or session", slog.String("error", oauthErr.Error()))
		return models.GrantSession{}, oauthErr
	}

	if oauthErr = validateAuthorizationCodeGrantRequest(req, authenticatedClient, session); oauthErr != nil {
		ctx.Logger.Debug("invalid parameters for the token request", slog.String("error", oauthErr.Error()))
		return models.GrantSession{}, oauthErr
	}

	ctx.Logger.Debug("fetch the token model")
	grantModel, err := ctx.GrantModelManager.Get(authenticatedClient.DefaultGrantModelId)
	if err != nil {
		ctx.Logger.Debug("error while loading the token model", slog.String("error", err.Error()))
		return models.GrantSession{}, issues.NewOAuthError(constants.InternalError, "could not load token model")
	}
	ctx.Logger.Debug("the token model was loaded successfully")

	grantSession := grantModel.GenerateGrantSession(NewAuthorizationCodeGrantOptions(ctx, req, session))
	err = nil
	if shouldCreateGrantSessionForAuthorizationCodeGrant(grantSession) {
		ctx.Logger.Debug("create token session")
		err = ctx.GrantSessionManager.CreateOrUpdate(grantSession)
	}
	if err != nil {
		return models.GrantSession{}, issues.NewOAuthError(constants.InternalError, "grant session not created")
	}

	return grantSession, nil
}

func preValidateAuthorizationCodeGrantRequest(req models.TokenRequest) issues.OAuthError {
	if req.AuthorizationCode == "" || unit.AnyNonEmpty(req.RefreshToken, req.Scope) {
		return issues.NewOAuthError(constants.InvalidRequest, "invalid parameter for authorization code grant")
	}

	// RFC 7636. "...with a minimum length of 43 characters and a maximum length of 128 characters."
	codeVerifierLengh := len(req.CodeVerifier)
	if req.CodeVerifier != "" && (codeVerifierLengh < 43 || codeVerifierLengh > 128) {
		return issues.NewOAuthError(constants.InvalidRequest, "invalid code verifier")
	}

	return nil
}

func validateAuthorizationCodeGrantRequest(req models.TokenRequest, client models.Client, session models.AuthnSession) issues.OAuthError {

	if !client.IsGrantTypeAllowed(constants.AuthorizationCodeGrant) {
		return issues.NewOAuthError(constants.UnauthorizedClient, "invalid grant type")
	}

	if session.ClientId != client.Id {
		return issues.NewOAuthError(constants.InvalidGrant, "the authorization code was not issued to the client")
	}

	if session.IsAuthorizationCodeExpired() {
		return issues.NewOAuthError(constants.InvalidGrant, "the authorization code is expired")
	}

	if session.RedirectUri != req.RedirectUri {
		return issues.NewOAuthError(constants.InvalidGrant, "invalid redirect_uri")
	}

	// If the session was created with a code challenge, the token request must contain the right code verifier.
	codeChallengeMethod := session.CodeChallengeMethod
	if codeChallengeMethod == "" {
		codeChallengeMethod = constants.PlainCodeChallengeMethod
	}
	if session.CodeChallenge != "" && (req.CodeVerifier == "" || !unit.IsPkceValid(req.CodeVerifier, session.CodeChallenge, codeChallengeMethod)) {
		return issues.NewOAuthError(constants.InvalidRequest, "invalid pkce")
	}

	return nil
}

func getAuthenticatedClientAndSession(ctx utils.Context, req models.TokenRequest) (models.Client, models.AuthnSession, issues.OAuthError) {

	ctx.Logger.Debug("get the session using the authorization code")
	sessionResultCh := make(chan utils.ResultChannel)
	go getSessionByAuthorizationCode(ctx, req.AuthorizationCode, sessionResultCh)

	ctx.Logger.Debug("get the client while the session is being loaded")
	authenticatedClient, err := GetAuthenticatedClient(ctx, req.ClientAuthnRequest)
	if err != nil {
		ctx.Logger.Debug("error while loading the client", slog.String("error", err.Error()))
		return models.Client{}, models.AuthnSession{}, err
	}
	ctx.Logger.Debug("the client was loaded successfully")

	ctx.Logger.Debug("wait for the session")
	sessionResult := <-sessionResultCh
	session, err := sessionResult.Result.(models.AuthnSession), sessionResult.Err
	if err != nil {
		ctx.Logger.Debug("error while loading the session", slog.String("error", err.Error()))
		return models.Client{}, models.AuthnSession{}, err
	}
	ctx.Logger.Debug("the session was loaded successfully")

	return authenticatedClient, session, nil
}

func getSessionByAuthorizationCode(ctx utils.Context, authorizationCode string, ch chan<- utils.ResultChannel) {
	session, err := ctx.AuthnSessionManager.GetByAuthorizationCode(authorizationCode)
	if err != nil {
		ch <- utils.ResultChannel{
			Result: models.AuthnSession{},
			Err:    issues.NewWrappingOAuthError(err, constants.InvalidGrant, "invalid authorization code"),
		}
	}

	// The session must be used only once when requesting a token.
	// By deleting it, we prevent replay attacks.
	err = ctx.AuthnSessionManager.Delete(session.Id)
	if err != nil {
		ch <- utils.ResultChannel{
			Result: models.AuthnSession{},
			Err:    issues.NewWrappingOAuthError(err, constants.InternalError, "could not delete session"),
		}
	}

	ch <- utils.ResultChannel{
		Result: session,
		Err:    nil,
	}
}

func shouldCreateGrantSessionForAuthorizationCodeGrant(grantSession models.GrantSession) bool {
	// We only need to create a token session for the authorization code grant when the token is not self-contained
	// (i.e. it is a refecence token), when the refresh token is issued or the the openid scope was requested
	// in which case the client can later request information about the user.
	return grantSession.TokenFormat == constants.Opaque || grantSession.RefreshToken != "" || slices.Contains(grantSession.Scopes, constants.OpenIdScope)
}

func NewAuthorizationCodeGrantOptions(ctx utils.Context, req models.TokenRequest, session models.AuthnSession) models.GrantOptions {
	return models.GrantOptions{
		GrantType: constants.AuthorizationCodeGrant,
		Scopes:    unit.SplitStringWithSpaces(session.Scope),
		Subject:   session.Subject,
		ClientId:  session.ClientId,
		TokenOptions: models.TokenOptions{
			DpopJwt:               req.DpopJwt,
			DpopSigningAlgorithms: ctx.DpopSigningAlgorithms,
			AdditionalTokenClaims: session.AdditionalTokenClaims,
		},
		IdTokenOptions: models.IdTokenOptions{
			Nonce:                   session.Nonce,
			AdditionalIdTokenClaims: session.AdditionalIdTokenClaims,
		},
	}
}
