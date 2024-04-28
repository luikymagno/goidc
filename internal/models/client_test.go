package models

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/luikymagno/auth-server/internal/unit"
	"github.com/luikymagno/auth-server/internal/unit/constants"
	"golang.org/x/crypto/bcrypt"
)

func TestNoneAuthenticatorValidInfo(t *testing.T) {
	authenticator := NoneClientAuthenticator{}
	req := ClientAuthnRequest{
		ClientIdPost: "client_id",
	}

	if !authenticator.IsAuthenticated(req) {
		t.Error("The client should be authenticated")
	}
}

func TestSecretClientAuthenticatorValidInfo(t *testing.T) {

	clientSecretSalt := "random_salt"
	clientSecret := "password"
	hashedClientSecret, _ := bcrypt.GenerateFromPassword([]byte(clientSecretSalt+clientSecret), 0)
	authenticator := SecretPostClientAuthenticator{
		Salt:         clientSecretSalt,
		HashedSecret: string(hashedClientSecret),
	}
	req := ClientAuthnRequest{
		ClientIdPost:     "client_id",
		ClientSecretPost: clientSecret,
	}

	if !authenticator.IsAuthenticated(req) {
		t.Error("The client should be authenticated")
	}
}

func TestSecretClientAuthenticatorInvalidInfo(t *testing.T) {
	clientSecretSalt := "random_salt"
	clientSecret := "password"
	hashedClientSecret, _ := bcrypt.GenerateFromPassword([]byte(clientSecretSalt+clientSecret), 0)
	authenticator := SecretPostClientAuthenticator{
		Salt:         clientSecretSalt,
		HashedSecret: string(hashedClientSecret),
	}
	req := ClientAuthnRequest{
		ClientIdPost:     "client_id",
		ClientSecretPost: "invalid_secret",
	}

	if authenticator.IsAuthenticated(req) {
		t.Error("The client should not be authenticated")
	}
}

func TestPrivateKeyJWTClientAuthenticatorValidInfo(t *testing.T) {

	// When
	host := "https://example.com"

	keyId := "0afee142-a0af-4410-abcc-9f2d44ff45b5"
	jwksBytes, _ := json.Marshal(
		map[string]any{
			"keys": []any{
				map[string]any{
					"kty": "oct",
					"kid": keyId,
					"alg": "HS256",
					"k":   "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ",
				},
			},
		},
	)
	var jwks jose.JSONWebKeySet
	json.Unmarshal(jwksBytes, &jwks)
	authenticator := PrivateKeyJwtClientAuthenticator{
		PublicJwks:               jwks,
		ExpectedAudience:         host,
		MaxAssertionLifetimeSecs: 60,
	}

	clientId := "random_client_id"
	createdAtTimestamp := unit.GetTimestampNow()
	signer, _ := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.SignatureAlgorithm(jwks.Keys[0].Algorithm), Key: jwks.Keys[0].Key},
		(&jose.SignerOptions{}).WithType("jwt").WithHeader("kid", keyId),
	)
	claims := map[string]any{
		string(constants.IssuerClaim):   clientId,
		string(constants.SubjectClaim):  clientId,
		string(constants.AudienceClaim): host,
		string(constants.IssuedAtClaim): createdAtTimestamp,
		string(constants.ExpiryClaim):   createdAtTimestamp + 59,
	}
	assertion, _ := jwt.Signed(signer).Claims(claims).Serialize()

	req := ClientAuthnRequest{
		ClientIdPost:        clientId,
		ClientAssertionType: constants.JWTBearerAssertion,
		ClientAssertion:     assertion,
	}

	// Then
	isAuthenticated := authenticator.IsAuthenticated(req)

	// Assert
	if !isAuthenticated {
		t.Error("The client should be authenticated")
	}
}

func TestAreScopesAllowed(t *testing.T) {
	client := Client{
		Scopes: []string{"scope1", "scope2", "scope3"},
	}
	testCases := []struct {
		requestedScopes []string
		expectedResult  bool
	}{
		{[]string{"scope1", "scope3"}, true},
		{[]string{"invalid_scope", "scope3"}, false},
	}

	for _, testCase := range testCases {
		t.Run(
			fmt.Sprintf("requested scopes: %v", testCase.requestedScopes),
			func(t *testing.T) {
				if client.AreScopesAllowed(testCase.requestedScopes) != testCase.expectedResult {
					t.Errorf("the scopes %v should be valid? %v", testCase.requestedScopes, testCase.expectedResult)
				}
			},
		)
	}
}

func TestAreResponseTypesAllowed(t *testing.T) {
	client := Client{
		ResponseTypes: []constants.ResponseType{constants.CodeResponse},
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
					t.Errorf("the response types %v should be valid? %v", testCase.requestedResponseType, testCase.expectedResult)
				}
			},
		)
	}
}

func TestIsGrantTypeAllowed(t *testing.T) {
	client := Client{
		GrantTypes: []constants.GrantType{constants.ClientCredentialsGrant},
	}
	testCases := []struct {
		requestedGrantType constants.GrantType
		expectedResult     bool
	}{
		{constants.ClientCredentialsGrant, true},
		{constants.AuthorizationCodeGrant, false},
	}

	for _, testCase := range testCases {
		t.Run(
			fmt.Sprint(testCase.requestedGrantType),
			func(t *testing.T) {
				if client.IsGrantTypeAllowed(testCase.requestedGrantType) != testCase.expectedResult {
					t.Errorf("the grant type %v should be allowed? %v", testCase.requestedGrantType, testCase.expectedResult)
				}
			},
		)
	}
}

func TestIsRedirectUriAllowed(t *testing.T) {
	client := Client{
		RedirectUris: []string{"https://example.com/callback", "http://example.com?param=value"},
	}
	testCases := []struct {
		redirectUri    string
		expectedResult bool
	}{
		{"https://example.com/callback", true},
		{"https://example.com/invalid", false},
	}

	for _, testCase := range testCases {
		t.Run(
			fmt.Sprint(testCase.redirectUri),
			func(t *testing.T) {
				if client.IsRedirectUriAllowed(testCase.redirectUri) != testCase.expectedResult {
					t.Errorf("the redirect URI %v should be allowed? %v", testCase.redirectUri, testCase.expectedResult)
				}
			},
		)
	}
}
