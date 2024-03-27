package models

import "github.com/luikymagno/auth-server/internal/unit/constants"

type GrantInfo struct {
	GrantType           constants.GrantType
	AuthenticatedClient Client
	TokenModel          TokenModel
	Scopes              []string
	AuthorizationCode   string
	RedirectUri         string
}

type TokenContextInfo struct {
	Subject  string
	ClientId string
	Scopes   []string
}

type Token struct {
	Id                 string
	TokenModelId       string
	TokenString        string
	RefreshToken       string
	ExpiresInSecs      int
	CreatedAtTimestamp int
	TokenContextInfo
}

type ClientAuthnRequest struct {
	ClientId     string `form:"client_id" binding:"required"`
	ClientSecret string `form:"client_secret"`
}

type TokenRequest struct {
	ClientAuthnRequest
	GrantType         constants.GrantType `form:"grant_type" binding:"required"`
	Scope             string              `form:"scope"`
	AuthorizationCode string              `form:"code"`
	RedirectUri       string              `form:"redirect_uri"`
	RefreshToken      string              `form:"refresh_token"`
}

func (req TokenRequest) IsValid() error {
	//TODO
	return nil
}

type TokenResponse struct {
	AccessToken  string              `json:"access_token"`
	RefreshToken string              `json:"refresh_token,omitempty"`
	ExpiresIn    int                 `json:"expires_in"`
	TokenType    constants.TokenType `json:"token_type"`
	Scope        string              `json:"scope,omitempty"`
}

type AuthorizeRequest struct {
	ClientId     string `form:"client_id" binding:"required"`
	RedirectUri  string `form:"redirect_uri"`
	Scope        string `form:"scope"`
	ResponseType string `form:"response_type"`
	State        string `form:"state"`
	RequestUri   string `form:"request_uri"`
}

func (req AuthorizeRequest) IsValid() error {
	//TODO
	return nil
}

type PARRequest struct {
	ClientAuthnRequest
	RedirectUri  string `form:"redirect_uri" binding:"required"`
	Scope        string `form:"scope" binding:"required"`
	ResponseType string `form:"response_type" binding:"required"`
	State        string `form:"state"`
	RequestUri   string `form:"request_uri"` // It is here only to make sure the client doesn't pass it during the request.
}

func (req PARRequest) ToAuthorizeRequest() AuthorizeRequest {
	return AuthorizeRequest{
		ClientId:     req.ClientId,
		RedirectUri:  req.RedirectUri,
		Scope:        req.Scope,
		ResponseType: req.ResponseType,
		State:        req.State,
		RequestUri:   "", // Make sure the request URI is set as its null value here.
	}
}

type PARResponse struct {
	RequestUri string `json:"request_uri"`
	ExpiresIn  int    `json:"expires_in"`
}
