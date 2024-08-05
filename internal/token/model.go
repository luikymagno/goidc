package token

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/luikyv/go-oidc/internal/authn"
	"github.com/luikyv/go-oidc/pkg/goidc"
)

type Token struct {
	ID                    string
	Format                goidc.TokenFormat
	Value                 string
	Type                  goidc.TokenType
	JWKThumbprint         string
	CertificateThumbprint string
}

type IDTokenOptions struct {
	Subject                 string
	AdditionalIDTokenClaims map[string]any
	// These values here below are intended to be hashed and placed in the ID token.
	// Then, the ID token can be used as a detached signature for the implicit grant.
	AccessToken       string
	AuthorizationCode string
	State             string
}

func newIDTokenOptions(grantOpts goidc.GrantOptions) IDTokenOptions {
	return IDTokenOptions{
		Subject:                 grantOpts.Subject,
		AdditionalIDTokenClaims: grantOpts.AdditionalIDTokenClaims,
	}
}

type DPoPJWTValidationOptions struct {
	// AccessToken should be filled when the DPoP "ath" claim is expected and should be validated.
	AccessToken   string
	JWKThumbprint string
}

type dpopJWTClaims struct {
	HTTPMethod      string `json:"htm"`
	HTTPURI         string `json:"htu"`
	AccessTokenHash string `json:"ath"`
}

type tokenRequest struct {
	GrantType         goidc.GrantType
	Scopes            string
	AuthorizationCode string
	RedirectURI       string
	RefreshToken      string
	CodeVerifier      string
	authn.ClientAuthnRequest
}

func newTokenRequest(req *http.Request) tokenRequest {
	return tokenRequest{
		ClientAuthnRequest: authn.NewClientAuthnRequest(req),
		GrantType:          goidc.GrantType(req.PostFormValue("grant_type")),
		Scopes:             req.PostFormValue("scope"),
		AuthorizationCode:  req.PostFormValue("code"),
		RedirectURI:        req.PostFormValue("redirect_uri"),
		RefreshToken:       req.PostFormValue("refresh_token"),
		CodeVerifier:       req.PostFormValue("code_verifier"),
	}
}

type tokenResponse struct {
	AccessToken          string                      `json:"access_token"`
	IDToken              string                      `json:"id_token,omitempty"`
	RefreshToken         string                      `json:"refresh_token,omitempty"`
	ExpiresIn            int                         `json:"expires_in"`
	TokenType            goidc.TokenType             `json:"token_type"`
	Scopes               string                      `json:"scope,omitempty"`
	AuthorizationDetails []goidc.AuthorizationDetail `json:"authorization_details,omitempty"`
}

type resultChannel struct {
	Result any
	Err    goidc.OAuthError
}

type tokenIntrospectionRequest struct {
	authn.ClientAuthnRequest
	Token         string
	TokenTypeHint goidc.TokenTypeHint
}

func newTokenIntrospectionRequest(req *http.Request) tokenIntrospectionRequest {
	return tokenIntrospectionRequest{
		ClientAuthnRequest: authn.NewClientAuthnRequest(req),
		Token:              req.PostFormValue("token"),
		TokenTypeHint:      goidc.TokenTypeHint(req.PostFormValue("token_type_hint")),
	}
}

func NewGrantSession(grantOptions goidc.GrantOptions, token Token) *goidc.GrantSession {
	timestampNow := goidc.TimestampNow()
	return &goidc.GrantSession{
		ID:                          uuid.New().String(),
		TokenID:                     token.ID,
		JWKThumbprint:               token.JWKThumbprint,
		ClientCertificateThumbprint: token.CertificateThumbprint,
		CreatedAtTimestamp:          timestampNow,
		LastTokenIssuedAtTimestamp:  timestampNow,
		ExpiresAtTimestamp:          timestampNow + grantOptions.TokenLifetimeSecs,
		ActiveScopes:                grantOptions.GrantedScopes,
		GrantOptions:                grantOptions,
	}
}
