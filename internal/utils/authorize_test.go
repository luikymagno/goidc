package utils_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/luikymagno/auth-server/internal/crud/mock"
	"github.com/luikymagno/auth-server/internal/issues"
	"github.com/luikymagno/auth-server/internal/models"
	"github.com/luikymagno/auth-server/internal/unit/constants"
	"github.com/luikymagno/auth-server/internal/utils"
)

func TestInitAuthenticationNoClientFound(t *testing.T) {

	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()

	// Then
	err := utils.InitAuthentication(ctx, models.AuthorizeRequest{ClientId: "invalid_client_id"})

	// Assert
	if err == nil {
		t.Error("InitAuthentication should not find any client")
	}
	var notFoundErr issues.EntityNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Error("InitAuthentication should not find any client")
	}
}

func TestInitAuthenticationInvalidRedirectUri(t *testing.T) {
	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()

	// Then
	err := utils.InitAuthentication(ctx, models.AuthorizeRequest{
		ClientId:    utils.ValidClient.Id,
		RedirectUri: "https://invalid.com",
	})

	// Assert
	var jsonErr issues.JsonError
	if err == nil || !errors.As(err, &jsonErr) {
		t.Error("the redirect URI should not be valid")
	}

	if jsonErr.ErrorCode != constants.InvalidRequest {
		t.Errorf("invalid error code: %s", jsonErr.ErrorCode)
	}
}

func TestInitAuthenticationInvalidScope(t *testing.T) {
	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()

	// Then
	err := utils.InitAuthentication(ctx, models.AuthorizeRequest{
		ClientId:     utils.ValidClient.Id,
		RedirectUri:  utils.ValidClient.RedirectUris[0],
		Scope:        "invalid_scope",
		ResponseType: string(constants.Code),
	})

	// Assert
	var redirectErr issues.RedirectError
	if err == nil || !errors.As(err, &redirectErr) {
		t.Error("the scope should not be valid")
	}

	if redirectErr.ErrorCode != constants.InvalidScope {
		t.Errorf("invalid error code: %s", redirectErr.ErrorCode)
	}
}

func TestInitAuthenticationInvalidResponseType(t *testing.T) {
	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()

	// Then
	err := utils.InitAuthentication(ctx, models.AuthorizeRequest{
		ClientId:     utils.ValidClient.Id,
		RedirectUri:  utils.ValidClient.RedirectUris[0],
		Scope:        strings.Join(utils.ValidClient.Scopes, " "),
		ResponseType: string(constants.IdToken),
	})

	// Assert
	var redirectErr issues.RedirectError
	if err == nil || !errors.As(err, &redirectErr) {
		t.Error("the response type should not be allowed")
	}

	if redirectErr.ErrorCode != constants.InvalidRequest {
		t.Errorf("invalid error code: %s", redirectErr.ErrorCode)
	}
}

func TestInitAuthenticationNoPolicyAvailable(t *testing.T) {
	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()

	// Then
	err := utils.InitAuthentication(ctx, models.AuthorizeRequest{
		ClientId:     utils.ValidClient.Id,
		RedirectUri:  utils.ValidClient.RedirectUris[0],
		Scope:        strings.Join(utils.ValidClient.Scopes, " "),
		ResponseType: string(constants.Code),
	})

	// Assert
	if err == nil {
		t.Error("no policy should be available")
	}
}

func TestInitAuthenticationPolicyEndsWithError(t *testing.T) {
	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()
	firstStep := models.NewStep(
		"init_step",
		models.FinishFlowSuccessfullyStep,
		models.FinishFlowWithFailureStep,
		func(as *models.AuthnSession, ctx *gin.Context) constants.AuthnStatus {
			return constants.Failure
		},
	)
	models.AddPolicy(models.AuthnPolicy{
		Id:              "policy_id",
		FirstStep:       firstStep,
		IsAvailableFunc: func(c models.Client, ctx *gin.Context) bool { return true },
	})

	// Then
	err := utils.InitAuthentication(ctx, models.AuthorizeRequest{
		ClientId:     utils.ValidClient.Id,
		RedirectUri:  utils.ValidClient.RedirectUris[0],
		Scope:        strings.Join(utils.ValidClient.Scopes, " "),
		ResponseType: string(constants.Code),
	})

	// Assert
	if err != nil {
		t.Errorf("no error should happen: %s", err.Error())
	}

	redirectUrl := ctx.RequestContext.Writer.Header().Get("Location")
	if !strings.Contains(redirectUrl, "error") {
		t.Errorf("the policy should finish redirecting with error. redirect URL: %s", redirectUrl)
	}

	sessionManager, _ := ctx.CrudManager.AuthnSessionManager.(*mock.MockedAuthnSessionManager)
	sessions := make([]models.AuthnSession, 0, len(sessionManager.Sessions))
	for _, s := range sessionManager.Sessions {
		sessions = append(sessions, s)
	}
	if len(sessions) != 0 {
		t.Error("no authentication session should remain")
	}

}

func TestInitAuthenticationPolicyEndsInProgress(t *testing.T) {
	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()
	firstStep := models.NewStep(
		"init_step",
		models.FinishFlowSuccessfullyStep,
		models.FinishFlowWithFailureStep,
		func(as *models.AuthnSession, ctx *gin.Context) constants.AuthnStatus {
			return constants.InProgress
		},
	)
	models.AddPolicy(models.AuthnPolicy{
		Id:              "policy_id",
		FirstStep:       firstStep,
		IsAvailableFunc: func(c models.Client, ctx *gin.Context) bool { return true },
	})

	// Then
	err := utils.InitAuthentication(ctx, models.AuthorizeRequest{
		ClientId:     utils.ValidClient.Id,
		RedirectUri:  utils.ValidClient.RedirectUris[0],
		Scope:        strings.Join(utils.ValidClient.Scopes, " "),
		ResponseType: string(constants.Code),
	})

	// Assert
	if err != nil {
		t.Errorf("no error should happen: %s", err.Error())
	}

	responseStatus := ctx.RequestContext.Writer.Status()
	if responseStatus != http.StatusOK {
		t.Errorf("invalid status code for in progress status: %v", responseStatus)
	}

	sessionManager, _ := ctx.CrudManager.AuthnSessionManager.(*mock.MockedAuthnSessionManager)
	sessions := make([]models.AuthnSession, 0, len(sessionManager.Sessions))
	for _, s := range sessionManager.Sessions {
		sessions = append(sessions, s)
	}
	if len(sessions) != 1 {
		t.Error("the should be only one authentication session")
	}

	session := sessions[0]
	if session.CallbackId == "" {
		t.Error("the callback ID was not filled")
	}
	if session.AuthorizationCode != "" {
		t.Error("the authorization code cannot be generated if the flow is still in progress")
	}
	if session.StepId != firstStep.Id {
		t.Errorf("the current step ID: %s is not as expected", session.StepId)
	}

}

func TestInitAuthenticationPolicyEndsWithSuccess(t *testing.T) {
	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()
	firstStep := models.NewStep(
		"init_step",
		models.FinishFlowSuccessfullyStep,
		models.FinishFlowWithFailureStep,
		func(as *models.AuthnSession, ctx *gin.Context) constants.AuthnStatus {
			return constants.Success
		},
	)
	models.AddPolicy(models.AuthnPolicy{
		Id:              "policy_id",
		FirstStep:       firstStep,
		IsAvailableFunc: func(c models.Client, ctx *gin.Context) bool { return true },
	})

	// Then
	err := utils.InitAuthentication(ctx, models.AuthorizeRequest{
		ClientId:     utils.ValidClient.Id,
		RedirectUri:  utils.ValidClient.RedirectUris[0],
		Scope:        strings.Join(utils.ValidClient.Scopes, " "),
		ResponseType: string(constants.Code),
	})

	// Assert
	if err != nil {
		t.Errorf("no error should happen: %s", err.Error())
	}

	sessionManager, _ := ctx.CrudManager.AuthnSessionManager.(*mock.MockedAuthnSessionManager)
	sessions := make([]models.AuthnSession, 0, len(sessionManager.Sessions))
	for _, s := range sessionManager.Sessions {
		sessions = append(sessions, s)
	}
	if len(sessions) != 1 {
		t.Error("the should be only one authentication session")
	}

	session := sessions[0]
	if session.AuthorizationCode == "" {
		t.Error("the authorization code should be filled when the policy ends successfully")
	}

	redirectUrl := ctx.RequestContext.Writer.Header().Get("Location")
	if !strings.Contains(redirectUrl, fmt.Sprintf("code=%s", session.AuthorizationCode)) {
		t.Errorf("the policy should finish redirecting with error. redirect URL: %s", redirectUrl)
	}

}

func TestContinueAuthenticationFindsSession(t *testing.T) {

	// When
	ctx, tearDown := utils.SetUp()
	defer tearDown()
	firstStep := models.NewStep(
		"init_step",
		models.FinishFlowSuccessfullyStep,
		models.FinishFlowWithFailureStep,
		func(as *models.AuthnSession, ctx *gin.Context) constants.AuthnStatus {
			return constants.InProgress
		},
	)

	callbackId := "random_callback_id"
	ctx.CrudManager.AuthnSessionManager.CreateOrUpdate(models.AuthnSession{
		StepId:     firstStep.Id,
		CallbackId: callbackId,
	})

	// Then
	err := utils.ContinueAuthentication(ctx, callbackId)

	// Assert
	if err != nil {
		t.Errorf("no error should happen: %s", err.Error())
	}

	sessionManager, _ := ctx.CrudManager.AuthnSessionManager.(*mock.MockedAuthnSessionManager)
	sessions := make([]models.AuthnSession, 0, len(sessionManager.Sessions))
	for _, s := range sessionManager.Sessions {
		sessions = append(sessions, s)
	}
	if len(sessions) != 1 {
		t.Error("the should be only one authentication session")
	}
}