package utils_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/luikymagno/auth-server/internal/crud"
	"github.com/luikymagno/auth-server/internal/issues"
	"github.com/luikymagno/auth-server/internal/models"
	"github.com/luikymagno/auth-server/internal/unit/constants"
	"github.com/luikymagno/auth-server/internal/utils"
)

func TestPushAuthorizationShouldRejectUnauthenticatedClient(t *testing.T) {
	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()

	// Then
	_, err := utils.PushAuthorization(ctx, models.PARRequest{
		ClientId:     utils.ValidClientId,
		ClientSecret: "invalid_password",
	})

	// Assert
	var jsonError issues.JsonError
	if err == nil || !errors.As(err, &jsonError) {
		t.Error("the client should not be authenticated")
		return
	}
	if jsonError.ErrorCode != constants.AccessDenied {
		t.Errorf("invalid error code: %s", jsonError.ErrorCode)
		return
	}
}

func TestPushAuthorizationWhenRequestUriIsSent(t *testing.T) {
	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()
	clientCh := make(chan crud.ClientGetResult, 1)
	ctx.CrudManager.ClientManager.Get(utils.ValidClientId, clientCh)
	client := (<-clientCh).Client

	// Then
	_, err := utils.PushAuthorization(ctx, models.PARRequest{
		ClientId:     utils.ValidClientId,
		ClientSecret: utils.ValidClientSecret,
		RedirectUri:  client.RedirectUris[0],
		Scope:        strings.Join(client.Scopes, " "),
		ResponseType: string(constants.Code),
		RequestUri:   "not_empty",
	})

	// Assert
	var jsonError issues.JsonError
	if err == nil || !errors.As(err, &jsonError) {
		t.Error("the request must not contain a request_uri")
		return
	}
	if jsonError.ErrorCode != constants.InvalidRequest {
		t.Errorf("invalid error code: %s", jsonError.ErrorCode)
		return
	}
}

func TestPushAuthorizationShouldGenerateRequestUri(t *testing.T) {
	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()
	clientCh := make(chan crud.ClientGetResult, 1)
	ctx.CrudManager.ClientManager.Get(utils.ValidClientId, clientCh)
	client := (<-clientCh).Client

	// Then
	requestUri, err := utils.PushAuthorization(ctx, models.PARRequest{
		ClientId:     utils.ValidClientId,
		ClientSecret: utils.ValidClientSecret,
		RedirectUri:  client.RedirectUris[0],
		Scope:        strings.Join(client.Scopes, " "),
		ResponseType: string(constants.Code),
	})

	// Assert
	if err != nil {
		t.Errorf("an error happened: %s", err.Error())
		return
	}
	if requestUri == "" {
		t.Error("the request_uri cannot be empty")
		return
	}

	sessions := utils.GetSessionsFromMock(ctx)
	if len(sessions) != 1 {
		t.Error("the should be only one authentication session")
		return
	}

	session := sessions[0]
	if session.RequestUri != requestUri {
		t.Error("the request URI informed is not the same in the session")
		return
	}
}
