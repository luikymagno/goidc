package oauth

import (
	"github.com/luikymagno/auth-server/internal/issues"
	"github.com/luikymagno/auth-server/internal/models"
	"github.com/luikymagno/auth-server/internal/unit"
	"github.com/luikymagno/auth-server/internal/unit/constants"
	"github.com/luikymagno/auth-server/internal/utils"
)

func HandleUserInfoRequest(ctx utils.Context, token string) (models.GrantSession, issues.OAuthError) {

	tokenId, oauthErr := utils.GetTokenId(ctx, token)
	if oauthErr != nil {
		return models.GrantSession{}, oauthErr
	}

	grantSession, err := ctx.GrantSessionManager.GetByTokenId(tokenId)
	if err != nil {
		return models.GrantSession{}, issues.NewOAuthError(constants.InvalidRequest, "invalid invalid token")
	}

	if grantSession.IsExpired() {
		return models.GrantSession{}, issues.NewOAuthError(constants.InvalidRequest, "token expired")
	}

	if !unit.ScopesContainsOpenId(grantSession.Scopes) {
		return models.GrantSession{}, issues.NewOAuthError(constants.InvalidRequest, "invalid scope")
	}

	return grantSession, nil
}
