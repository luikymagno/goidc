package constants

import (
	"net/http"
)

type GrantType string

const (
	ClientCredentials GrantType = "client_credentials"
	AuthorizationCode GrantType = "authorization_code"
	RefreshToken      GrantType = "refresh_token"
)

type ResponseType string

const (
	Code    ResponseType = "code"
	IdToken ResponseType = "id_token"
)

type ClientAuthnType string

const (
	None          ClientAuthnType = "none"
	ClientSecret  ClientAuthnType = "client_secret"
	PrivateKeyJWT ClientAuthnType = "private_key_jwt"
)

type ClientAssertionType string

const (
	JWTBearer ClientAssertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
)

type TokenType string

const (
	Bearer TokenType = "Bearer"
)

type SigningAlgorithm string

const (
	HS256 SigningAlgorithm = "HS256"
)

type KeyType string

const (
	Octet KeyType = "oct" // Octet sequence (used to represent symmetric keys).
)

type ErrorCode string

const (
	AccessDenied   ErrorCode = "access_denied"
	InvalidRequest ErrorCode = "invalid_request"
	InvalidScope   ErrorCode = "invalid_scope"
)

var ErrorCodeToStatusCode map[ErrorCode]int = map[ErrorCode]int{
	AccessDenied:   http.StatusForbidden,
	InvalidRequest: http.StatusBadRequest,
	InvalidScope:   http.StatusBadRequest,
}

type Header string

const CorrelationIdHeader Header = "X-Correlation-ID"
