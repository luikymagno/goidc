package token

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/luikyv/go-oidc/internal/authn"
	"github.com/luikyv/go-oidc/internal/oidc"
	"github.com/luikyv/go-oidc/pkg/goidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGrantCreationShouldNotFindClient(t *testing.T) {
	// Given.
	ctx := oidc.NewTestContext(t)

	// When.
	_, err := HandleTokenCreation(ctx, tokenRequest{
		ClientAuthnRequest: authn.ClientAuthnRequest{
			ClientID: "invalid_client_id",
		},
		GrantType: goidc.GrantClientCredentials,
		Scopes:    "scope1",
	})

	// Then.
	assert.NotNil(t, err, "the client should not be found")
}

func TestHandleGrantCreationShouldRejectUnauthenticatedClient(t *testing.T) {
	// Given.
	client := oidc.NewTestClient(t)
	client.AuthnMethod = goidc.ClientAuthnSecretPost

	ctx := oidc.NewTestContext(t)
	require.Nil(t, ctx.SaveClient(client))

	// When.
	_, err := HandleTokenCreation(ctx, tokenRequest{
		ClientAuthnRequest: authn.ClientAuthnRequest{
			ClientID:     client.ID,
			ClientSecret: "invalid_password",
		},
		GrantType: goidc.GrantClientCredentials,
		Scopes:    "scope1",
	})

	// Then.
	require.NotNil(t, err, "the client should not be authenticated")

	var oauthErr oidc.Error
	require.ErrorAs(t, err, &oauthErr)
	assert.Equal(t, oidc.ErrorCodeInvalidClient, oauthErr.Code(), "invalid error code")
}

func TestHandleGrantCreationWithDPoP(t *testing.T) {
	// Given.
	ctx := oidc.NewTestContext(t)
	ctx.Host = "https://example.com"
	ctx.DPoPIsEnabled = true
	ctx.DPoPLifetimeSecs = 9999999999999
	ctx.DPoPSignatureAlgorithms = []jose.SignatureAlgorithm{jose.ES256}
	ctx.Request().Header.Set(goidc.HeaderDPoP, "eyJ0eXAiOiJkcG9wK2p3dCIsImFsZyI6IkVTMjU2IiwiandrIjp7Imt0eSI6IkVDIiwiY3J2IjoiUC0yNTYiLCJ4IjoiYVRtMk95eXFmaHFfZk5GOVVuZXlrZG0yX0dCZnpZVldDNEI1Wlo1SzNGUSIsInkiOiI4eFRhUERFTVRtNXM1d1MzYmFvVVNNcU01R0VJWDFINzMwX1hqV2lRaGxRIn19.eyJqdGkiOiItQndDM0VTYzZhY2MybFRjIiwiaHRtIjoiUE9TVCIsImh0dSI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdG9rZW4iLCJpYXQiOjE1NjIyNjUyOTZ9.AzzSCVYIimNZyJQefZq7cF252PukDvRrxMqrrcH6FFlHLvpXyk9j8ybtS36GHlnyH_uuy2djQphfyHGeDfxidQ")
	ctx.Request().Method = http.MethodPost
	ctx.Request().RequestURI = "/token"

	req := tokenRequest{
		ClientAuthnRequest: authn.ClientAuthnRequest{
			ClientID:     oidc.TestClientID,
			ClientSecret: oidc.TestClientSecret,
		},
		GrantType: goidc.GrantClientCredentials,
		Scopes:    "scope1",
	}

	// When.
	tokenResp, err := HandleTokenCreation(ctx, req)

	// Then.
	assert.Nil(t, err)
	claims := oidc.UnsafeClaims(t, tokenResp.AccessToken, []jose.SignatureAlgorithm{jose.PS256, jose.RS256})

	require.Contains(t, claims, "cnf")
	confirmation := claims["cnf"].(map[string]any)
	require.Contains(t, confirmation, "jkt")
	assert.Equal(t, "BABEGlQNVH1K8KXO7qLKtvUFhAadQ5-dVGBfDfelwhQ", confirmation["jkt"])
}

func TestIsJWS(t *testing.T) {
	testCases := []struct {
		jws         string
		shouldBeJWS bool
	}{
		{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", true},
		{"not a jwt", false},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case %v", i+1), func(t *testing.T) {
			assert.Equal(t, testCase.shouldBeJWS, IsJWS(testCase.jws))
		})
	}
}

func TestIsJWE(t *testing.T) {
	testCases := []struct {
		jwe         string
		shouldBeJWE bool
	}{
		{"eyJhbGciOiJSU0EtT0FFUCIsImVuYyI6IkEyNTZHQ00ifQ.OKOawDo13gRp2ojaHV7LFpZcgV7T6DVZKTyKOMTYUmKoTCVJRgckCL9kiMT03JGeipsEdY3mx_etLbbWSrFr05kLzcSr4qKAq7YN7e9jwQRb23nfa6c9d-StnImGyFDbSv04uVuxIp5Zms1gNxKKK2Da14B8S4rzVRltdYwam_lDp5XnZAYpQdb76FdIKLaVmqgfwX7XWRxv2322i-vDxRfqNzo_tETKzpVLzfiwQyeyPGLBIO56YJ7eObdv0je81860ppamavo35UgoRdbYaBcoh9QcfylQr66oc6vFWXRcZ_ZT2LawVCWTIy3brGPi6UklfCpIMfIjf7iGdXKHzg.48V1_ALb6US04U3b.5eym8TW_c8SuK0ltJ3rpYIzOeDQz7TALvtu6UG9oMo4vpzs9tX_EFShS8iB7j6jiSdiwkIr3ajwQzaBtQD_.XFBoMYUZodetZdvTiFvSkQ", true},
		{"not a jwt", false},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case %v", i+1), func(t *testing.T) {
			assert.Equal(t, testCase.shouldBeJWE, IsJWE(testCase.jwe))
		})
	}
}
