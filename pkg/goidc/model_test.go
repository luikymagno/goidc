package goidc_test

import (
	"testing"

	"github.com/luikyv/go-oidc/pkg/goidc"
	"github.com/stretchr/testify/assert"
)

func TestAddTokenClaims_HappyPath(t *testing.T) {
	// Given.
	tokenOptions := goidc.TokenOptions{}

	// When.
	tokenOptions.AddTokenClaims(map[string]any{
		"claim": "value",
	})
	// Then.
	assert.Equal(t, "value", tokenOptions.AdditionalTokenClaims["claim"], "the claim was not added")

	// When.
	tokenOptions.AddTokenClaims(map[string]any{
		"claim": "value",
	})
	// Then.
	assert.Equal(t, "value", tokenOptions.AdditionalTokenClaims["claim"], "the claim was not added")
}

func TestAuthorizationParameters_Merge_HappyPath(t *testing.T) {
	// Given.
	insideParams := goidc.AuthorizationParameters{
		RedirectURI:          "https:example1.com",
		State:                "random_state",
		AuthorizationDetails: []goidc.AuthorizationDetail{},
	}
	outsideParams := goidc.AuthorizationParameters{
		RedirectURI: "https:example2.com",
		Nonce:       "random_nonce",
		Claims:      &goidc.ClaimsObject{},
	}

	// When.
	mergedParams := insideParams.Merge(outsideParams)

	// Then.
	assert.Equal(t, "https:example1.com", mergedParams.RedirectURI, "the redirect URI is not as expected")
	assert.Equal(t, "random_state", mergedParams.State, "the redirect URI is not as expected")
	assert.Equal(t, "random_nonce", mergedParams.Nonce, "the nonce is not as expected")
	assert.NotNil(t, mergedParams.AuthorizationDetails, "the authorization details are not as expected")
	assert.NotNil(t, mergedParams.Claims, "the claims are not as expected")
}

func TestAuthorizationDetail_GetProperties_HappyPath(t *testing.T) {
	// Given.
	authDetails := goidc.AuthorizationDetail{
		"type":       "random_type",
		"identifier": "random_identifier",
		"actions":    []string{"random_action"},
	}

	// Then.
	assert.Equal(t, "random_type", authDetails.Type(), "type not as expected")
	assert.Equal(t, "random_identifier", authDetails.Identifier(), "identifier not as expected")
	assert.Contains(t, authDetails.Actions(), "random_action", "action not as expected")
}
