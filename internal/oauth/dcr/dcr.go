package dcr

import (
	"github.com/luikymagno/goidc/internal/utils"
	"github.com/luikymagno/goidc/pkg/goidc"
)

func CreateClient(
	ctx utils.Context,
	dynamicClient utils.DynamicClientRequest,
) (
	utils.DynamicClientResponse,
	goidc.OAuthError,
) {
	setCreationDefaults(ctx, &dynamicClient)
	if err := validateDynamicClientRequest(ctx, dynamicClient); err != nil {
		return utils.DynamicClientResponse{}, err
	}

	ctx.ExecuteDCRPlugin(&dynamicClient.ClientMetaInfo)
	if err := validateDynamicClientRequest(ctx, dynamicClient); err != nil {
		return utils.DynamicClientResponse{}, err
	}

	newClient := newClient(dynamicClient)
	if err := ctx.CreateOrUpdateClient(newClient); err != nil {
		return utils.DynamicClientResponse{}, goidc.NewOAuthError(goidc.InternalError, err.Error())
	}

	return utils.DynamicClientResponse{
		ID:                      dynamicClient.ID,
		RegistrationURI:         getClientRegistrationURI(ctx, dynamicClient.ID),
		RegistrationAccessToken: dynamicClient.RegistrationAccessToken,
		Secret:                  dynamicClient.Secret,
		ClientMetaInfo:          dynamicClient.ClientMetaInfo,
	}, nil
}

func UpdateClient(
	ctx utils.Context,
	dynamicClient utils.DynamicClientRequest,
) (
	utils.DynamicClientResponse,
	goidc.OAuthError,
) {
	client, err := getProtectedClient(ctx, dynamicClient)
	if err != nil {
		return utils.DynamicClientResponse{}, err
	}

	setUpdateDefaults(ctx, client, &dynamicClient)
	if err := validateDynamicClientRequest(ctx, dynamicClient); err != nil {
		return utils.DynamicClientResponse{}, err
	}

	ctx.ExecuteDCRPlugin(&dynamicClient.ClientMetaInfo)
	if err := validateDynamicClientRequest(ctx, dynamicClient); err != nil {
		return utils.DynamicClientResponse{}, err
	}

	updatedClient := newClient(dynamicClient)
	if err := ctx.CreateOrUpdateClient(updatedClient); err != nil {
		return utils.DynamicClientResponse{}, goidc.NewOAuthError(goidc.InternalError, err.Error())
	}

	return utils.DynamicClientResponse{
		ID:                      dynamicClient.ID,
		RegistrationURI:         getClientRegistrationURI(ctx, dynamicClient.ID),
		RegistrationAccessToken: dynamicClient.RegistrationAccessToken,
		Secret:                  dynamicClient.Secret,
		ClientMetaInfo:          dynamicClient.ClientMetaInfo,
	}, nil
}

func GetClient(
	ctx utils.Context,
	dynamicClientRequest utils.DynamicClientRequest,
) (
	utils.DynamicClientResponse,
	goidc.OAuthError,
) {

	client, err := getProtectedClient(ctx, dynamicClientRequest)
	if err != nil {
		return utils.DynamicClientResponse{}, err
	}

	return utils.DynamicClientResponse{
		ID:              client.ID,
		RegistrationURI: getClientRegistrationURI(ctx, client.ID),
		ClientMetaInfo:  client.ClientMetaInfo,
	}, nil
}

func DeleteClient(
	ctx utils.Context,
	dynamicClientRequest utils.DynamicClientRequest,
) goidc.OAuthError {
	_, err := getProtectedClient(ctx, dynamicClientRequest)
	if err != nil {
		return err
	}

	if err := ctx.DeleteClient(dynamicClientRequest.ID); err != nil {
		return goidc.NewOAuthError(goidc.InternalError, err.Error())
	}
	return nil
}
