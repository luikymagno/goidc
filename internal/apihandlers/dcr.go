package apihandlers

import (
	"encoding/json"
	"net/http"

	"github.com/luikymagno/goidc/internal/constants"
	"github.com/luikymagno/goidc/internal/models"
	"github.com/luikymagno/goidc/internal/oauth/dcr"
	"github.com/luikymagno/goidc/internal/utils"
)

func HandleDynamicClientCreation(ctx utils.Context) {
	var req models.DynamicClientRequest
	if err := json.NewDecoder(ctx.Request.Body).Decode(&req); err != nil {
		bindErrorToResponse(ctx, err)
		return
	}

	initialAccessToken, _ := ctx.GetBearerToken()
	req.InitialAccessToken = initialAccessToken

	resp, err := dcr.CreateClient(ctx, req)
	if err != nil {
		bindErrorToResponse(ctx, err)
		return
	}

	if err := ctx.WriteJson(resp, http.StatusCreated); err != nil {
		bindErrorToResponse(ctx, err)
	}
}

func HandleDynamicClientUpdate(ctx utils.Context) {
	var req models.DynamicClientRequest
	if err := json.NewDecoder(ctx.Request.Body).Decode(&req); err != nil {
		bindErrorToResponse(ctx, err)
		return
	}

	token, ok := ctx.GetBearerToken()
	if !ok {
		bindErrorToResponse(ctx, models.NewOAuthError(constants.AccessDenied, "no token found"))
		return
	}

	req.Id = ctx.Request.PathValue("client_id")
	req.RegistrationAccessToken = token
	resp, err := dcr.UpdateClient(ctx, req)
	if err != nil {
		bindErrorToResponse(ctx, err)
		return
	}

	if err := ctx.WriteJson(resp, http.StatusOK); err != nil {
		bindErrorToResponse(ctx, err)
	}
}

func HandleDynamicClientRetrieve(ctx utils.Context) {
	token, ok := ctx.GetBearerToken()
	if !ok {
		bindErrorToResponse(ctx, models.NewOAuthError(constants.AccessDenied, "no token found"))
		return
	}

	req := models.DynamicClientRequest{
		Id:                      ctx.Request.PathValue("client_id"),
		RegistrationAccessToken: token,
	}

	resp, err := dcr.GetClient(ctx, req)
	if err != nil {
		bindErrorToResponse(ctx, err)
		return
	}

	if err := ctx.WriteJson(resp, http.StatusOK); err != nil {
		bindErrorToResponse(ctx, err)
	}
}

func HandleDynamicClientDelete(ctx utils.Context) {
	token, ok := ctx.GetBearerToken()
	if !ok {
		bindErrorToResponse(ctx, models.NewOAuthError(constants.AccessDenied, "no token found"))
		return
	}

	req := models.DynamicClientRequest{
		Id:                      ctx.Request.PathValue("client_id"),
		RegistrationAccessToken: token,
	}

	if err := dcr.DeleteClient(ctx, req); err != nil {
		bindErrorToResponse(ctx, err)
		return
	}

	ctx.Response.WriteHeader(http.StatusNoContent)
}
