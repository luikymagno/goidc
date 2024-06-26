package authorize_test

import (
	"testing"

	"github.com/luikymagno/goidc/internal/oauth/authorize"
	"github.com/luikymagno/goidc/internal/utils"
	"github.com/luikymagno/goidc/pkg/goidc"
)

func TestValidateAuthorizationRequest(t *testing.T) {
	client := utils.GetTestClient()

	var cases = []struct {
		Name                string
		Req                 utils.AuthorizationRequest
		ClientModifyFunc    func(client goidc.Client) goidc.Client
		ShouldBeValid       bool
		ShouldRedirectError bool
	}{
		{
			"valid_oauth_request",
			utils.AuthorizationRequest{
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI:  client.RedirectURIS[0],
					ResponseType: goidc.CodeResponse,
					ResponseMode: goidc.QueryResponseMode,
					Scopes:       client.Scopes,
				},
			},
			func(client goidc.Client) goidc.Client {
				return client
			},
			true,
			false,
		},
		{
			"valid_openid_request",
			utils.AuthorizationRequest{
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI:  client.RedirectURIS[0],
					ResponseType: goidc.CodeResponse,
					Scopes:       goidc.OpenIDScope,
				},
			},
			func(client goidc.Client) goidc.Client {
				return client
			},
			true,
			false,
		},
		{
			"oauth_request_invalid_response_type",
			utils.AuthorizationRequest{
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI:  client.RedirectURIS[0],
					ResponseType: goidc.CodeResponse,
					Scopes:       goidc.OpenIDScope,
				},
			},
			func(client goidc.Client) goidc.Client {
				client.ResponseTypes = []goidc.ResponseType{}
				return client
			},
			false,
			true,
		},
		{
			"oauth_request_missing_response_type",
			utils.AuthorizationRequest{
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI: client.RedirectURIS[0],
					Scopes:      goidc.OpenIDScope,
				},
			},
			func(client goidc.Client) goidc.Client {
				return client
			},
			false,
			true,
		},
		{
			"oauth_request_invalid_scope",
			utils.AuthorizationRequest{
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI:  client.RedirectURIS[0],
					ResponseType: goidc.CodeResponse,
					Scopes:       "invalid_scope",
				},
			},
			func(client goidc.Client) goidc.Client {
				return client
			},
			false,
			true,
		},
		{
			"oauth_request_invalid_redirect_uri",
			utils.AuthorizationRequest{
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI:  "https://invalid.com",
					ResponseType: goidc.CodeResponse,
					Scopes:       client.Scopes,
				},
			},
			func(client goidc.Client) goidc.Client {
				return client
			},
			false,
			false,
		},
	}

	for _, c := range cases {
		t.Run(
			c.Name,
			func(t *testing.T) {
				// Then.
				err := authorize.ValidateAuthorizationRequest(
					utils.GetDummyTestContext(),
					c.Req,
					c.ClientModifyFunc(client),
				)

				// Assert.
				isValid := err == nil
				if isValid != c.ShouldBeValid {
					t.Errorf("expected: %v - actual: %v - error: %s", c.ShouldBeValid, isValid, err)
					return
				}

				_, ok := err.(goidc.OAuthRedirectError)
				if c.ShouldRedirectError && !ok {
					t.Errorf("error is not of type redirect. Error: %v", err)
				}
			},
		)
	}

}

func TestValidateAuthorizationRequestWithPAR(t *testing.T) {
	client := utils.GetTestClient()

	var cases = []struct {
		Name                string
		Req                 utils.AuthorizationRequest
		Session             goidc.AuthnSession
		ClientModifyFunc    func(client goidc.Client) goidc.Client
		ShouldBeValid       bool
		ShouldRedirectError bool
	}{
		{
			"valid_oauth_request",
			utils.AuthorizationRequest{
				ClientID: client.ID,
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI:  client.RedirectURIS[0],
					ResponseType: goidc.CodeResponse,
					ResponseMode: goidc.QueryResponseMode,
					Scopes:       client.Scopes,
				},
			},
			goidc.AuthnSession{
				ClientID:           client.ID,
				ExpiresAtTimestamp: goidc.GetTimestampNow() + 1,
			},
			func(client goidc.Client) goidc.Client {
				return client
			},
			true,
			false,
		},
		{
			"valid_openid_request",
			utils.AuthorizationRequest{
				ClientID: client.ID,
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI:  client.RedirectURIS[0],
					ResponseType: goidc.CodeAndIDTokenResponse,
					ResponseMode: goidc.FragmentResponseMode,
					Scopes:       goidc.OpenIDScope,
					Nonce:        "random_nonce",
				},
			},
			goidc.AuthnSession{
				ClientID:           client.ID,
				ExpiresAtTimestamp: goidc.GetTimestampNow() + 1,
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI: client.RedirectURIS[0],
				},
			},
			func(client goidc.Client) goidc.Client {
				return client
			},
			true,
			false,
		},
	}

	for _, c := range cases {
		t.Run(
			c.Name,
			func(t *testing.T) {
				// Then.
				err := authorize.ValidateAuthorizationRequestWithPAR(
					utils.GetDummyTestContext(),
					c.Req,
					c.Session,
					c.ClientModifyFunc(client),
				)

				// Assert.
				isValid := err == nil
				if isValid != c.ShouldBeValid {
					t.Errorf("expected: %v - actual: %v - error: %s", c.ShouldBeValid, isValid, err)
					return
				}

				_, ok := err.(goidc.OAuthRedirectError)
				if c.ShouldRedirectError && !ok {
					t.Errorf("error is not of type redirect. Error: %v", err)
				}
			},
		)
	}
}

func TestValidateAuthorizationRequestWithJAR(t *testing.T) {
	client := utils.GetTestClient()

	var cases = []struct {
		Name                string
		Req                 utils.AuthorizationRequest
		JAR                 utils.AuthorizationRequest
		ClientModifyFunc    func(client goidc.Client) goidc.Client
		ShouldBeValid       bool
		ShouldRedirectError bool
	}{
		{
			"valid_oauth_request",
			utils.AuthorizationRequest{
				ClientID: client.ID,
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI:  client.RedirectURIS[0],
					ResponseType: goidc.CodeResponse,
					ResponseMode: goidc.QueryResponseMode,
					Scopes:       client.Scopes,
					Nonce:        "random_nonce",
				},
			},
			utils.AuthorizationRequest{
				ClientID:                client.ID,
				AuthorizationParameters: goidc.AuthorizationParameters{},
			},
			func(client goidc.Client) goidc.Client {
				return client
			},
			true,
			false,
		},
		{
			"valid_openid_request",
			utils.AuthorizationRequest{
				ClientID: client.ID,
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI:  client.RedirectURIS[0],
					ResponseType: goidc.CodeAndIDTokenResponse,
					ResponseMode: goidc.FragmentResponseMode,
					Scopes:       goidc.OpenIDScope,
				},
			},
			utils.AuthorizationRequest{
				ClientID: client.ID,
				AuthorizationParameters: goidc.AuthorizationParameters{
					RedirectURI: client.RedirectURIS[0],
					Nonce:       "random_nonce",
				},
			},
			func(client goidc.Client) goidc.Client {
				return client
			},
			true,
			false,
		},
		{
			"client_id_does_not_match",
			utils.AuthorizationRequest{
				ClientID: client.ID,
			},
			utils.AuthorizationRequest{
				ClientID: "invalid_client_id",
			},
			func(client goidc.Client) goidc.Client {
				return client
			},
			false,
			false,
		},
	}

	for _, c := range cases {
		t.Run(
			c.Name,
			func(t *testing.T) {
				// Then.
				err := authorize.ValidateAuthorizationRequestWithJAR(
					utils.GetDummyTestContext(),
					c.Req,
					c.JAR,
					c.ClientModifyFunc(client),
				)

				// Assert.
				isValid := err == nil
				if isValid != c.ShouldBeValid {
					t.Errorf("expected: %v - actual: %v - error: %s", c.ShouldBeValid, isValid, err)
					return
				}

				_, ok := err.(goidc.OAuthRedirectError)
				if c.ShouldRedirectError && !ok {
					t.Errorf("error is not of type redirect. Error: %v", err)
				}
			},
		)
	}
}
