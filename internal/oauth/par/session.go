package par

import (
	"github.com/luikymagno/auth-server/internal/issues"
	"github.com/luikymagno/auth-server/internal/models"
	"github.com/luikymagno/auth-server/internal/oauth/authorize"
	"github.com/luikymagno/auth-server/internal/unit/constants"
	"github.com/luikymagno/auth-server/internal/utils"
)

func initValidAuthnSession(
	ctx utils.Context,
	req models.PushedAuthorizationRequest,
	client models.Client,
) (
	models.AuthnSession,
	issues.OAuthError,
) {

	if authorize.ShouldInitAuthnSessionWithJar(ctx, req.AuthorizationParameters, client) {
		return initValidAuthnSessionWithJar(ctx, req, client)
	}

	return initValidSimpleAuthnSession(ctx, req, client)
}

func initValidSimpleAuthnSession(
	ctx utils.Context,
	req models.PushedAuthorizationRequest,
	client models.Client,
) (
	models.AuthnSession,
	issues.OAuthError,
) {
	if err := validatePar(ctx, req, client); err != nil {
		ctx.Logger.Info("request has invalid params")
		return models.AuthnSession{}, err
	}

	session := models.NewSession(req.AuthorizationParameters, client)
	return session, nil
}

func initValidAuthnSessionWithJar(
	ctx utils.Context,
	req models.PushedAuthorizationRequest,
	client models.Client,
) (
	models.AuthnSession,
	issues.OAuthError,
) {
	jar, err := extractJarFromRequest(ctx, req, client)
	if err != nil {
		return models.AuthnSession{}, err
	}

	if err := validateParWithJar(ctx, req, jar, client); err != nil {
		return models.AuthnSession{}, err
	}

	session := models.NewSession(jar.AuthorizationParameters, client)
	return session, nil
}

func extractJarFromRequest(
	ctx utils.Context,
	req models.PushedAuthorizationRequest,
	client models.Client,
) (
	models.AuthorizationRequest,
	issues.OAuthError,
) {
	if req.RequestObject == "" {
		return models.AuthorizationRequest{}, issues.NewOAuthError(constants.InvalidRequest, "invalid request object")
	}

	return utils.ExtractJarFromRequestObject(ctx, req.RequestObject, client)
}
