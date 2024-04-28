package models

import (
	"errors"
	"slices"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/luikymagno/auth-server/internal/issues"
	"github.com/luikymagno/auth-server/internal/unit"
	"github.com/luikymagno/auth-server/internal/unit/constants"
)

type IdTokenContext struct {
	Nonce                   string
	AdditionalIdTokenClaims map[string]string
	// These values here below are intended to be hashed and placed in the ID token.
	// Then, the ID token can be used as a detached signature for the implict grant.
	AccessToken       string
	AuthorizationCode string
	State             string
}

type TokenContext struct {
	Scopes                []string
	GrantType             constants.GrantType
	AdditionalTokenClaims map[string]string
}

type GrantContext struct {
	Subject  string
	ClientId string
	TokenContext
	IdTokenContext
}

func NewClientCredentialsGrantContext(client Client, req TokenRequest) GrantContext {
	return GrantContext{
		Subject:  client.Id,
		ClientId: client.Id,
		TokenContext: TokenContext{
			Scopes:                unit.SplitStringWithSpaces(req.Scope),
			GrantType:             constants.ClientCredentialsGrant,
			AdditionalTokenClaims: make(map[string]string),
		},
		IdTokenContext: IdTokenContext{
			AdditionalIdTokenClaims: make(map[string]string),
		},
	}
}

func NewAuthorizationCodeGrantContext(session AuthnSession) GrantContext {
	return GrantContext{
		Subject:  session.Subject,
		ClientId: session.ClientId,
		TokenContext: TokenContext{
			Scopes:                session.Scopes,
			GrantType:             constants.AuthorizationCodeGrant,
			AdditionalTokenClaims: session.AdditionalTokenClaims,
		},
		IdTokenContext: IdTokenContext{
			Nonce:                   session.Nonce,
			AdditionalIdTokenClaims: session.AdditionalIdTokenClaims,
		},
	}
}

func NewImplictGrantContext(session AuthnSession) GrantContext {
	return GrantContext{
		Subject:  session.Subject,
		ClientId: session.ClientId,
		TokenContext: TokenContext{
			Scopes:                session.Scopes,
			GrantType:             constants.ImplictGrant,
			AdditionalTokenClaims: session.AdditionalTokenClaims,
		},
		IdTokenContext: IdTokenContext{
			Nonce:                   session.Nonce,
			AdditionalIdTokenClaims: session.AdditionalIdTokenClaims,
		},
	}
}

func NewImplictGrantContextForIdToken(session AuthnSession, idToken IdTokenContext) GrantContext {
	return GrantContext{
		Subject:  session.Subject,
		ClientId: session.ClientId,
		TokenContext: TokenContext{
			Scopes:                session.Scopes,
			GrantType:             constants.ImplictGrant,
			AdditionalTokenClaims: session.AdditionalTokenClaims,
		},
		IdTokenContext: idToken,
	}
}

func NewRefreshTokenGrantContext(session GrantSession) GrantContext {
	return GrantContext{
		Subject:  session.Subject,
		ClientId: session.ClientId,
		TokenContext: TokenContext{
			Scopes:                session.Scopes,
			GrantType:             constants.RefreshTokenGrant,
			AdditionalTokenClaims: session.AdditionalTokenClaims,
		},
		IdTokenContext: IdTokenContext{
			Nonce:                   session.Nonce,
			AdditionalIdTokenClaims: session.AdditionalIdTokenClaims,
		},
	}
}

type ClientAuthnRequest struct {
	ClientIdBasicAuthn     string
	ClientSecretBasicAuthn string
	ClientIdPost           string                        `form:"client_id"`
	ClientSecretPost       string                        `form:"client_secret"`
	ClientAssertionType    constants.ClientAssertionType `form:"client_assertion_type"`
	ClientAssertion        string                        `form:"client_assertion"`
}

func (req ClientAuthnRequest) IsValid() error {

	// Either the client ID or the client assertion must be present to identity the client.
	if _, ok := req.getValidClientId(); !ok {
		return issues.OAuthError{
			ErrorCode:        constants.InvalidClient,
			ErrorDescription: "invalid client authentication",
		}
	}

	// Validate parameters for client secret basic authentication.
	if req.ClientSecretBasicAuthn != "" && (req.ClientIdBasicAuthn == "" || req.ClientSecretPost != "" || req.ClientAssertionType != "" || req.ClientAssertion != "") {
		return issues.OAuthError{
			ErrorCode:        constants.InvalidClient,
			ErrorDescription: "invalid client authentication",
		}
	}

	// Validate parameters for client secret post authentication.
	if req.ClientSecretPost != "" && (req.ClientIdPost == "" || req.ClientIdBasicAuthn != "" || req.ClientSecretBasicAuthn != "" || req.ClientAssertionType != "" || req.ClientAssertion != "") {
		return issues.OAuthError{
			ErrorCode:        constants.InvalidClient,
			ErrorDescription: "invalid client authentication",
		}
	}

	// Validate parameters for private key jwt authentication.
	if req.ClientAssertion != "" && (req.ClientAssertionType != constants.JWTBearerAssertion || req.ClientIdBasicAuthn != "" || req.ClientSecretBasicAuthn != "" || req.ClientSecretPost != "") {
		return issues.OAuthError{
			ErrorCode:        constants.InvalidClient,
			ErrorDescription: "invalid client authentication",
		}
	}

	return nil
}

func (req ClientAuthnRequest) getValidClientId() (string, bool) {
	clientIds := []string{}

	if req.ClientIdPost != "" {
		clientIds = append(clientIds, req.ClientIdPost)
	}

	if req.ClientIdBasicAuthn != "" {
		clientIds = append(clientIds, req.ClientIdBasicAuthn)
	}

	if req.ClientAssertion != "" {
		assertionClientId, ok := req.getClientIdFromAssertion()
		// If the assertion is passed, it must contain the client ID as its issuer.
		if !ok {
			return "", false
		}
		clientIds = append(clientIds, assertionClientId)
	}

	// All the client IDs present must be equal.
	if len(clientIds) == 0 || unit.Any(clientIds, func(clientId string) bool {
		return clientId != clientIds[0]
	}) {
		return "", false
	}

	return clientIds[0], true
}

func (req ClientAuthnRequest) getClientIdFromAssertion() (string, bool) {
	assertion, err := jwt.ParseSigned(req.ClientAssertion, constants.ClientSigningAlgorithms)
	if err != nil {
		return "", false
	}

	var claims map[constants.Claim]any
	assertion.UnsafeClaimsWithoutVerification(&claims)

	clientId, ok := claims[constants.IssuerClaim]
	if !ok {
		return "", false
	}

	clientIdAsString, ok := clientId.(string)
	if !ok {
		return "", false
	}

	return clientIdAsString, true
}

// This method is intended to be called only after the request is validated.
func (req ClientAuthnRequest) GetClientId() string {
	clientId, _ := req.getValidClientId()
	return clientId
}

type TokenRequest struct {
	ClientAuthnRequest
	GrantType         constants.GrantType `form:"grant_type" binding:"required"`
	Scope             string              `form:"scope"`
	AuthorizationCode string              `form:"code"`
	RedirectUri       string              `form:"redirect_uri"`
	RefreshToken      string              `form:"refresh_token"`
	CodeVerifier      string              `form:"code_verifier"`
}

func (req TokenRequest) IsValid() error {

	if err := req.ClientAuthnRequest.IsValid(); err != nil {
		return err
	}

	// RFC 7636. "...with a minimum length of 43 characters and a maximum length of 128 characters."
	codeVerifierLengh := len(req.CodeVerifier)
	if req.CodeVerifier != "" && (codeVerifierLengh < 43 || codeVerifierLengh > 128) {
		return errors.New("invalid code verifier")
	}

	// Validate parameters specific to each grant type.
	switch req.GrantType {
	case constants.ClientCredentialsGrant:
		if req.AuthorizationCode != "" || req.RedirectUri != "" || req.RefreshToken != "" {
			return errors.New("invalid parameter for client credentials grant")
		}
	case constants.AuthorizationCodeGrant:
		if req.AuthorizationCode == "" || req.RedirectUri == "" || req.RefreshToken != "" || req.Scope != "" {
			return errors.New("invalid parameter for authorization code grant")
		}
	case constants.RefreshTokenGrant:
		if req.RefreshToken == "" || req.AuthorizationCode != "" || req.RedirectUri != "" || req.Scope != "" {
			return errors.New("invalid parameter for refresh token grant")
		}
	default:
		return issues.OAuthError{
			ErrorCode:        constants.UnsupportedGrantType,
			ErrorDescription: "unsupported grant type",
		}
	}

	return nil
}

type TokenResponse struct {
	AccessToken  string              `json:"access_token"`
	IdToken      string              `json:"id_token,omitempty"`
	RefreshToken string              `json:"refresh_token,omitempty"`
	ExpiresIn    int                 `json:"expires_in"`
	TokenType    constants.TokenType `json:"token_type"`
	Scope        string              `json:"scope,omitempty"`
}

type BaseAuthorizeRequest struct {
	RedirectUri         string                        `form:"redirect_uri"`
	Request             string                        `form:"request"`
	Scope               string                        `form:"scope"`
	ResponseType        constants.ResponseType        `form:"response_type"`
	ResponseMode        constants.ResponseMode        `form:"response_mode"`
	State               string                        `form:"state"`
	CodeChallenge       string                        `form:"code_challenge"`
	CodeChallengeMethod constants.CodeChallengeMethod `form:"code_challenge_method"`
	RequestUri          string                        `form:"request_uri"`
	Nonce               string                        `form:"nonce"`
}

func (req BaseAuthorizeRequest) IsValid() error {

	if req.RequestUri != "" {
		return req.isValidWithPar()
	}

	return req.isValidWithoutPar()
}

func (req BaseAuthorizeRequest) isValidWithPar() error {
	// If the request URI is passed, all the other parameters must be empty.
	if unit.Any(
		[]string{req.Request, req.RedirectUri, req.Scope, string(req.ResponseType), string(req.ResponseMode), req.CodeChallenge, string(req.CodeChallengeMethod)},
		func(s string) bool { return s != "" },
	) {
		return errors.New("invalid parameter")
	}

	return nil
}

func (req BaseAuthorizeRequest) isValidWithoutPar() error {

	// If the request object is passed, all the other parameters must be empty.
	if req.Request != "" && unit.Any(
		[]string{req.Request, req.RedirectUri, req.Scope, string(req.ResponseType), string(req.ResponseMode), req.CodeChallenge, string(req.CodeChallengeMethod)},
		func(s string) bool { return s != "" },
	) {
		return errors.New("invalid parameter")
	}

	if unit.Any(
		[]string{req.RedirectUri, req.Scope, string(req.ResponseType)},
		func(s string) bool { return s == "" },
	) {
		return errors.New("invalid parameter")
	}

	if !slices.Contains(constants.ResponseTypes, req.ResponseType) {
		return errors.New("invalid response type")
	}

	if req.ResponseMode != "" && !slices.Contains(constants.ResponseModes, req.ResponseMode) {
		return errors.New("invalid response mode")
	}

	// Implict response types cannot be sent via query parameteres.
	if req.ResponseType.IsImplict() && req.ResponseMode.IsQuery() {
		return errors.New("invalid response mode for the chosen response type")
	}

	// Validate PKCE parameters.
	// The code challenge cannot be informed without the method and vice versa.
	if (req.CodeChallenge != "" && req.CodeChallengeMethod == "") || (req.CodeChallenge == "" && req.CodeChallengeMethod != "") {
		return errors.New("invalid parameters for PKCE")
	}

	return nil
}

type AuthorizeRequest struct {
	ClientId string `form:"client_id"`
	BaseAuthorizeRequest
}

func (req AuthorizeRequest) IsValid() error {
	if req.ClientId == "" {
		return errors.New("invalid parameter")
	}

	return req.BaseAuthorizeRequest.IsValid()
}

type PARRequest struct {
	ClientAuthnRequest
	BaseAuthorizeRequest
}

func (req PARRequest) IsValid() error {

	// As mentioned in https://datatracker.ietf.org/doc/html/rfc9126,
	// "...The client_id parameter is defined with the same semantics for both authorization requests
	// and requests to the token endpoint; as a required authorization request parameter,
	// it is similarly required in a pushed authorization request...""
	if req.ClientIdPost == "" {
		return errors.New("invalid parameter")
	}

	if err := req.ClientAuthnRequest.IsValid(); err != nil {
		return err
	}

	if req.RequestUri != "" {
		return errors.New("invalid parameter")
	}

	return req.BaseAuthorizeRequest.IsValid()
}

func (req PARRequest) ToAuthorizeRequest(client Client) AuthorizeRequest {
	return AuthorizeRequest{
		ClientId:             client.Id,
		BaseAuthorizeRequest: req.BaseAuthorizeRequest,
	}
}

type PARResponse struct {
	RequestUri string `json:"request_uri"`
	ExpiresIn  int    `json:"expires_in"`
}

type OpenIdConfiguration struct {
	Issuer                   string                            `json:"issuer"`
	AuthorizationEndpoint    string                            `json:"authorization_endpoint"`
	TokenEndpoint            string                            `json:"token_endpoint"`
	UserinfoEndpoint         string                            `json:"userinfo_endpoint"`
	JwksUri                  string                            `json:"jwks_uri"`
	ParEndpoint              string                            `json:"pushed_authorization_request_endpoint"`
	ParIsRequired            bool                              `json:"require_pushed_authorization_requests"`
	ResponseTypes            []constants.ResponseType          `json:"response_types_supported"`
	ResponseModes            []constants.ResponseMode          `json:"response_modes_supported"`
	GrantTypes               []constants.GrantType             `json:"grant_types_supported"`
	SubjectIdentifierTypes   []constants.SubjectIdentifierType `json:"subject_types_supported"`
	IdTokenSigningAlgorithms []jose.SignatureAlgorithm         `json:"id_token_signing_alg_values_supported"`
	ClientAuthnMethods       []constants.ClientAuthnType       `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported          []string                          `json:"scopes_supported"`
	JarmAlgorithms           []string                          `json:"authorization_signing_alg_values_supported"`
}

type RedirectResponse struct {
	ClientId     string
	RedirectUri  string
	ResponseType constants.ResponseType
	ResponseMode constants.ResponseMode
	Parameters   map[string]string
}

func NewRedirectResponseFromSession(session AuthnSession, params map[string]string) RedirectResponse {
	return RedirectResponse{
		ClientId:     session.ClientId,
		RedirectUri:  session.RedirectUri,
		Parameters:   params,
		ResponseType: session.ResponseType,
		ResponseMode: session.ResponseMode,
	}
}

func NewRedirectResponseFromRedirectError(err issues.OAuthRedirectError) RedirectResponse {
	errorParams := map[string]string{
		"error":             string(err.ErrorCode),
		"error_description": err.ErrorDescription,
	}
	if err.State != "" {
		errorParams["state"] = err.State
	}
	return RedirectResponse{
		ClientId:     err.ClientId,
		RedirectUri:  err.RedirectUri,
		Parameters:   errorParams,
		ResponseType: err.ResponseType,
		ResponseMode: err.ResponseMode,
	}
}
