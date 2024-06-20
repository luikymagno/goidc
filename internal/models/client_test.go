package models_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/luikymagno/goidc/internal/constants"
	"github.com/luikymagno/goidc/internal/models"
	"github.com/luikymagno/goidc/internal/unit"
	"golang.org/x/crypto/bcrypt"
)

func TestAreScopesAllowed(t *testing.T) {
	client := models.Client{
		ClientMetaInfo: models.ClientMetaInfo{
			Scopes: "scope1 scope2 scope3",
		},
	}
	testCases := []struct {
		requestedScopes string
		expectedResult  bool
	}{
		{"scope1 scope3", true},
		{"scope3 scope2", true},
		{"invalid_scope scope3", false},
	}

	for i, testCase := range testCases {
		t.Run(
			fmt.Sprintf("case %v", i),
			func(t *testing.T) {
				if client.AreScopesAllowed(testCase.requestedScopes) != testCase.expectedResult {
					t.Error(testCase)
				}
			},
		)
	}
}

func TestIsResponseTypeAllowed(t *testing.T) {
	client := models.Client{
		ClientMetaInfo: models.ClientMetaInfo{
			ResponseTypes: []constants.ResponseType{constants.CodeResponse},
		},
	}
	testCases := []struct {
		requestedResponseType constants.ResponseType
		expectedResult        bool
	}{
		{constants.CodeResponse, true},
		{constants.CodeAndIdTokenResponse, false},
	}

	for i, testCase := range testCases {
		t.Run(
			fmt.Sprintf("case %v", i),
			func(t *testing.T) {
				if client.IsResponseTypeAllowed(testCase.requestedResponseType) != testCase.expectedResult {
					t.Error(testCase)
				}
			},
		)
	}
}

func TestIsGrantTypeAllowed(t *testing.T) {
	client := models.Client{
		ClientMetaInfo: models.ClientMetaInfo{
			GrantTypes: []constants.GrantType{constants.ClientCredentialsGrant},
		},
	}
	testCases := []struct {
		requestedGrantType constants.GrantType
		expectedResult     bool
	}{
		{constants.ClientCredentialsGrant, true},
		{constants.AuthorizationCodeGrant, false},
	}

	for i, testCase := range testCases {
		t.Run(
			fmt.Sprintf("case %v", i),
			func(t *testing.T) {
				if client.IsGrantTypeAllowed(testCase.requestedGrantType) != testCase.expectedResult {
					t.Error(testCase)
				}
			},
		)
	}
}

func TestIsRedirectUriAllowed(t *testing.T) {
	client := models.Client{
		ClientMetaInfo: models.ClientMetaInfo{
			RedirectUris: []string{"https://example.com/callback", "http://example.com?param=value"},
		},
	}
	testCases := []struct {
		redirectUri    string
		expectedResult bool
	}{
		{"https://example.com/callback", true},
		{"https://example.com/callback?param=value", true},
		{"https://example.com/invalid", false},
	}

	for i, testCase := range testCases {
		t.Run(
			fmt.Sprintf("case %v", i),
			func(t *testing.T) {
				if client.IsRedirectUriAllowed(testCase.redirectUri) != testCase.expectedResult {
					t.Error(testCase)
				}
			},
		)
	}
}

func TestIsAuthorizationDetailTypeAllowed(t *testing.T) {
	// When.
	client := models.Client{}

	// Then.
	isValid := client.IsAuthorizationDetailTypeAllowed("random_type")

	// Assert.
	if !isValid {
		t.Error("when the client doesn't specify the detail types, any type should be accepted")
	}

	// When.
	client.AuthorizationDetailTypes = []string{"valid_type"}

	// Then.
	isValid = client.IsAuthorizationDetailTypeAllowed("valid_type")

	// Assert.
	if !isValid {
		t.Error("the client specified the detail types, so an allowed type should be valid")
	}

	// Then.
	isValid = client.IsAuthorizationDetailTypeAllowed("random_type")

	// Assert.
	if isValid {
		t.Error("the client specified the detail types, so a not allowed type shouldn't be valid")
	}
}

func TestIsRegistrationAccessTokenValid(t *testing.T) {
	registrationAccessToken := "random_token"
	hashedRegistrationAccessToken, _ := bcrypt.GenerateFromPassword([]byte(registrationAccessToken), bcrypt.DefaultCost)
	client := models.Client{
		HashedRegistrationAccessToken: string(hashedRegistrationAccessToken),
	}

	if client.IsRegistrationAccessTokenValid("invalid_token") {
		t.Errorf("the token should not be valid")
	}

	if !client.IsRegistrationAccessTokenValid(registrationAccessToken) {
		t.Errorf("the token should be valid")
	}
}

func TestGetPublicJwks(t *testing.T) {

	// When.
	jwk := unit.GetTestPrivatePs256Jwk("random_key_id")
	numberOfRequestsToJwksUri := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		numberOfRequestsToJwksUri++
		json.NewEncoder(w).Encode(jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{jwk},
		})
	}))

	client := models.Client{
		ClientMetaInfo: models.ClientMetaInfo{
			PublicJwksUri: server.URL,
			PublicJwks:    &jose.JSONWebKeySet{},
		},
	}

	// Then.
	jwks, err := client.GetPublicJwks()

	// Assert.
	if err != nil {
		t.Errorf("error fetching the JWKS")
		return
	}

	if numberOfRequestsToJwksUri != 1 {
		t.Errorf("the jwks uri should've been requested once")
	}

	if len(jwks.Keys) == 0 {
		t.Errorf("the jwks was not fetched")
	}

	// Then.
	jwks, err = client.GetPublicJwks()

	// Assert.
	if err != nil {
		t.Errorf("error fetching the JWKS the second time")
		return
	}

	if numberOfRequestsToJwksUri != 1 {
		t.Errorf("the jwks uri should've been cached and therefore requested only once")
	}

	if len(jwks.Keys) == 0 {
		t.Errorf("the jwks was not fetched")
	}
}
