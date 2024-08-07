package goidc

import (
	"encoding/json"
	"maps"
	"net/http"
	"reflect"
)

type WrapHandlerFunc func(nextHandler http.Handler) http.Handler

// DCRPluginFunc defines a function that will be executed during DCR and DCM.
// It can be used to modify the client and perform custom validations.
type DCRPluginFunc func(ctx Context, clientInfo *ClientMetaInfo)

type AuthorizeErrorPluginFunc func(ctx Context, err error) error

var (
	ScopeOpenID        = NewScope("openid")
	ScopeProfile       = NewScope("profile")
	ScopeEmail         = NewScope("email")
	ScopeAddress       = NewScope("address")
	ScopeOfflineAccess = NewScope("offline_access")
)

type ScopeMatchingFunc func(requestedScope string) bool

type Scope struct {
	// ID is the string representation of the scope.
	// Its value will be exported as is.
	ID string
	// Matches validates if a requested scope is valid.
	Matches ScopeMatchingFunc
}

// NewScope creates a scope where the validation logic is simple string comparison.
func NewScope(scope string) Scope {
	return Scope{
		ID: scope,
		Matches: func(requestedScope string) bool {
			return scope == requestedScope
		},
	}
}

// NewDynamicScope creates a scope with custom logic that will be used to validate
// the scopes requested by the client.
//
//	dynamicScope := NewDynamicScope(
//		"payment",
//		func(requestedScope string) bool {
//			return strings.HasPrefix(requestedScope, "payment")
//		},
//	)
//
//	// This results in true.
//	dynamicScope.Matches("payment:30")
func NewDynamicScope(
	scope string,
	matchingFunc ScopeMatchingFunc,
) Scope {
	return Scope{
		ID:      scope,
		Matches: matchingFunc,
	}
}

type TokenOptionsFunc func(client *Client, scopes string) (TokenOptions, error)

type TokenOptions struct {
	TokenFormat           TokenFormat    `json:"token_format"`
	TokenLifetimeSecs     int64          `json:"token_lifetime_secs"`
	JWTSignatureKeyID     string         `json:"token_signature_key_id,omitempty"`
	OpaqueTokenLength     int            `json:"opaque_token_length,omitempty"`
	AdditionalTokenClaims map[string]any `json:"additional_token_claims,omitempty"`
}

func (to *TokenOptions) AddTokenClaims(claims map[string]any) {
	if to.AdditionalTokenClaims == nil {
		to.AdditionalTokenClaims = map[string]any{}
	}
	maps.Copy(to.AdditionalTokenClaims, claims)
}

func NewJWTTokenOptions(
	// signatureKeyID is the ID of a signing key present in the server JWKS.
	signatureKeyID string,
	tokenLifetimeSecs int64,
) TokenOptions {
	return TokenOptions{
		TokenFormat:       TokenFormatJWT,
		TokenLifetimeSecs: tokenLifetimeSecs,
		JWTSignatureKeyID: signatureKeyID,
	}
}

func NewOpaqueTokenOptions(
	tokenLength int,
	tokenLifetimeSecs int64,
) TokenOptions {
	return TokenOptions{
		TokenFormat:       TokenFormatOpaque,
		TokenLifetimeSecs: tokenLifetimeSecs,
		OpaqueTokenLength: tokenLength,
	}
}

// AuthnFunc executes the user authentication logic.
type AuthnFunc func(Context, *AuthnSession) AuthnStatus

// SetUpAuthnFunc is responsible for deciding if the corresponding policy will
// be executed.
type SetUpAuthnFunc func(Context, *Client, *AuthnSession) bool

type AuthnPolicy struct {
	ID           string
	SetUp        SetUpAuthnFunc
	Authenticate AuthnFunc
}

// NewPolicy creates a policy that will be selected based on setUpFunc and that
// authenticates users with authnFunc.
func NewPolicy(
	id string,
	setUpFunc SetUpAuthnFunc,
	authnFunc AuthnFunc,
) AuthnPolicy {
	return AuthnPolicy{
		ID:           id,
		Authenticate: authnFunc,
		SetUp:        setUpFunc,
	}
}

type TokenInfo struct {
	IsActive                    bool
	TokenUsage                  TokenTypeHint
	Scopes                      string
	AuthorizationDetails        []AuthorizationDetail
	ClientID                    string
	Subject                     string
	ExpiresAtTimestamp          int64
	JWKThumbprint               string
	ClientCertificateThumbprint string
	AdditionalTokenClaims       map[string]any
}

func (info TokenInfo) MarshalJSON() ([]byte, error) {
	if !info.IsActive {
		return json.Marshal(map[string]any{
			"active": false,
		})
	}

	params := map[string]any{
		"active":      true,
		"token_usage": info.TokenUsage,
		ClaimSubject:  info.Subject,
		ClaimScope:    info.Scopes,
		ClaimClientID: info.ClientID,
		ClaimExpiry:   info.ExpiresAtTimestamp,
	}

	if info.AuthorizationDetails != nil {
		params[ClaimAuthorizationDetails] = info.AuthorizationDetails
	}

	confirmation := make(map[string]string)
	if info.JWKThumbprint != "" {
		confirmation["jkt"] = info.JWKThumbprint
	}
	if info.ClientCertificateThumbprint != "" {
		confirmation["x5t#S256"] = info.ClientCertificateThumbprint
	}
	if len(confirmation) != 0 {
		params["cnf"] = confirmation
	}

	for k, v := range info.AdditionalTokenClaims {
		params[k] = v
	}

	return json.Marshal(params)
}

type AuthorizationParameters struct {
	RequestURI           string                `json:"request_uri,omitempty" bson:"request_uri,omitempty"`
	RequestObject        string                `json:"request,omitempty" bson:"request,omitempty"`
	RedirectURI          string                `json:"redirect_uri,omitempty" bson:"redirect_uri,omitempty"`
	ResponseMode         ResponseMode          `json:"response_mode,omitempty" bson:"response_mode,omitempty"`
	ResponseType         ResponseType          `json:"response_type,omitempty" bson:"response_type,omitempty"`
	Scopes               string                `json:"scope,omitempty" bson:"scope,omitempty"`
	State                string                `json:"state,omitempty" bson:"state,omitempty"`
	Nonce                string                `json:"nonce,omitempty" bson:"nonce,omitempty"`
	CodeChallenge        string                `json:"code_challenge,omitempty" bson:"code_challenge,omitempty"`
	CodeChallengeMethod  CodeChallengeMethod   `json:"code_challenge_method,omitempty" bson:"code_challenge_method,omitempty"`
	Prompt               PromptType            `json:"prompt,omitempty" bson:"prompt,omitempty"`
	MaxAuthnAgeSecs      *int                  `json:"max_age,omitempty" bson:"max_age,omitempty"`
	Display              DisplayValue          `json:"display,omitempty" bson:"display,omitempty"`
	ACRValues            string                `json:"acr_values,omitempty" bson:"acr_values,omitempty"`
	Claims               *ClaimsObject         `json:"claims,omitempty" bson:"claims,omitempty"`
	AuthorizationDetails []AuthorizationDetail `json:"authorization_details,omitempty" bson:"authorization_details,omitempty"`
}

func (insideParams AuthorizationParameters) Merge(outsideParams AuthorizationParameters) AuthorizationParameters {
	params := AuthorizationParameters{
		RedirectURI:          nonEmptyOrDefault(insideParams.RedirectURI, outsideParams.RedirectURI),
		ResponseMode:         nonEmptyOrDefault(insideParams.ResponseMode, outsideParams.ResponseMode),
		ResponseType:         nonEmptyOrDefault(insideParams.ResponseType, outsideParams.ResponseType),
		Scopes:               nonEmptyOrDefault(insideParams.Scopes, outsideParams.Scopes),
		State:                nonEmptyOrDefault(insideParams.State, outsideParams.State),
		Nonce:                nonEmptyOrDefault(insideParams.Nonce, outsideParams.Nonce),
		CodeChallenge:        nonEmptyOrDefault(insideParams.CodeChallenge, outsideParams.CodeChallenge),
		CodeChallengeMethod:  nonEmptyOrDefault(insideParams.CodeChallengeMethod, outsideParams.CodeChallengeMethod),
		Prompt:               nonEmptyOrDefault(insideParams.Prompt, outsideParams.Prompt),
		MaxAuthnAgeSecs:      nonEmptyOrDefault(insideParams.MaxAuthnAgeSecs, outsideParams.MaxAuthnAgeSecs),
		Display:              nonEmptyOrDefault(insideParams.Display, outsideParams.Display),
		ACRValues:            nonEmptyOrDefault(insideParams.ACRValues, outsideParams.ACRValues),
		Claims:               nonNilOrDefault(insideParams.Claims, outsideParams.Claims),
		AuthorizationDetails: nonNilOrDefault(insideParams.AuthorizationDetails, outsideParams.AuthorizationDetails),
	}

	return params
}

func nonEmptyOrDefault[T any](s1 T, s2 T) T {
	if reflect.ValueOf(s1).String() == "" {
		return s2
	}

	return s1
}

func nonNilOrDefault[T any](s1 T, s2 T) T {
	if reflect.ValueOf(s1).IsNil() {
		return s2
	}

	return s1
}

type ClaimsObject struct {
	UserInfo map[string]ClaimObjectInfo `json:"userinfo"`
	IDToken  map[string]ClaimObjectInfo `json:"id_token"`
}

// UserInfoEssentials returns all the essentials claims requested by the client to be returned in the userinfo endpoint.
func (claims ClaimsObject) UserInfoEssentials() []string {
	return essentials(claims.UserInfo)
}

// IDTokenEssentials returns all the essentials claims requested by the client to be returned in the ID token.
func (claims ClaimsObject) IDTokenEssentials() []string {
	return essentials(claims.IDToken)
}

// UserInfoClaim returns the claim object info if present.
func (claims ClaimsObject) UserInfoClaim(claimName string) (ClaimObjectInfo, bool) {
	return claim(claimName, claims.UserInfo)
}

// IDTokenClaim returns the claim object info if present.
func (claims ClaimsObject) IDTokenClaim(claimName string) (ClaimObjectInfo, bool) {
	return claim(claimName, claims.IDToken)
}

func claim(claim string, claims map[string]ClaimObjectInfo) (ClaimObjectInfo, bool) {
	for name, claimInfo := range claims {
		if name == claim {
			return claimInfo, true
		}
	}
	return ClaimObjectInfo{}, false
}

func essentials(claims map[string]ClaimObjectInfo) []string {
	var essentialClaims []string
	for name, claim := range claims {
		if claim.IsEssential {
			essentialClaims = append(essentialClaims, name)
		}
	}
	return essentialClaims
}

type ClaimObjectInfo struct {
	IsEssential bool     `json:"essential"`
	Value       string   `json:"value"`
	Values      []string `json:"values"`
}

// Authorization details is a map instead of a struct, because its fields vary a lot depending on the use case.
// Some fields are well know so they are accessible as methods.
type AuthorizationDetail map[string]any

func (detail AuthorizationDetail) Type() string {
	return detail.string("type")
}

func (detail AuthorizationDetail) Identifier() string {
	return detail.string("identifier")
}

func (detail AuthorizationDetail) Locations() []string {
	return detail.stringSlice("locations")
}

func (detail AuthorizationDetail) Actions() []string {
	return detail.stringSlice("actions")
}

func (detail AuthorizationDetail) DataTypes() []string {
	return detail.stringSlice("datatypes")
}

func (detail AuthorizationDetail) stringSlice(key string) []string {
	value, ok := detail[key]
	if !ok {
		return nil
	}

	slice, ok := value.([]string)
	if !ok {
		return nil
	}

	return slice
}

func (detail AuthorizationDetail) string(key string) string {
	value, ok := detail[key]
	if !ok {
		return ""
	}

	s, ok := value.(string)
	if !ok {
		return ""
	}

	return s
}
