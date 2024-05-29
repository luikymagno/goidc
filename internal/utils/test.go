package utils

import (
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v4"
	"github.com/luikymagno/auth-server/internal/crud/inmemory"
	"github.com/luikymagno/auth-server/internal/models"
	"github.com/luikymagno/auth-server/internal/unit"
	"github.com/luikymagno/auth-server/internal/unit/constants"
)

const (
	TestHost string = "https://example.com"
)

func SetUpTest() (testCtx Context, tearDownTest func()) {
	// Create
	privateJwk := unit.GetTestPrivateRs256Jwk("rsa256_key")
	client := models.GetTestClientWithSecretPostAuthn()

	// Save
	testCtx = GetTestInMemoryContext(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{privateJwk}}, privateJwk.KeyID)
	testCtx.ClientManager.Create(client)

	return testCtx, func() {
		testCtx.ClientManager.Delete(client.Id)
	}
}

func GetTestInMemoryRequestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = &http.Request{}
	return ctx
}

func GetTestInMemoryContext(privateJWKS jose.JSONWebKeySet, tokenSignatureKeyId string) Context {
	return Context{
		Configuration: Configuration{
			Host:                    TestHost,
			ClientManager:           inmemory.NewInMemoryClientManager(),
			GrantSessionManager:     inmemory.NewInMemoryGrantSessionManager(),
			AuthnSessionManager:     inmemory.NewInMemoryAuthnSessionManager(),
			ParIsEnabled:            true,
			JarIsEnabled:            true,
			PrivateJwks:             privateJWKS,
			IdTokenSignatureKeyIds:  []string{tokenSignatureKeyId},
			DpopIsEnabled:           true,
			DpopSignatureAlgorithms: []jose.SignatureAlgorithm{jose.ES256, jose.RS256},
			DpopLifetimeSecs:        99999999999,
			Policies:                []AuthnPolicy{},
			GetTokenOptions: func(client models.Client, scopes string) models.TokenOptions {
				return models.TokenOptions{
					ExpiresInSecs:     60,
					TokenFormat:       constants.JwtTokenFormat,
					JwtSignatureKeyId: tokenSignatureKeyId,
				}
			},
		},
		RequestContext: GetTestInMemoryRequestContext(),
		Logger:         slog.Default(),
	}
}

func GetDummyTestContext() Context {
	return Context{
		Configuration: Configuration{
			Host: TestHost,
		},
		Logger: slog.Default(),
	}
}

func GetSessionsFromTestContext(ctx Context) []models.AuthnSession {
	sessionManager, _ := ctx.AuthnSessionManager.(*inmemory.InMemoryAuthnSessionManager)
	sessions := make([]models.AuthnSession, 0, len(sessionManager.Sessions))
	for _, s := range sessionManager.Sessions {
		sessions = append(sessions, s)
	}

	return sessions
}

func GetGrantSessionsFromTestContext(ctx Context) []models.GrantSession {
	tokenManager, _ := ctx.GrantSessionManager.(*inmemory.InMemoryGrantSessionManager)
	tokens := make([]models.GrantSession, 0, len(tokenManager.GrantSessions))
	for _, t := range tokenManager.GrantSessions {
		tokens = append(tokens, t)
	}

	return tokens
}