package constants

import "github.com/go-jose/go-jose/v4"

const CallbackIdLength int = 20

const RequestUriLength int = 20

const PARLifetimeSecs int = 60

const AuthorizationCodeLifetimeSecs int = 60

const AuthorizationCodeLength int = 30

const RefreshTokenLength int = 30

var ClientSigningAlgorithms []jose.SignatureAlgorithm = []jose.SignatureAlgorithm{
	jose.RS256,
}

const Charset string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type AuthnStatus string

const (
	Success    AuthnStatus = "success"
	InProgress AuthnStatus = "in_progress"
	Failure    AuthnStatus = "failure"
)

const CorrelationIdKey string = "correlation_id"

type TokenFormat string

const (
	JWT    TokenFormat = "jwt"
	Opaque TokenFormat = "opaque"
)

type EndpointPath string

const (
	WellKnownEndpoint                  EndpointPath = "/.well-known/openid-configuration"
	JsonWebKeySetEndpoint              EndpointPath = "/jwks"
	PushedAuthorizationRequestEndpoint EndpointPath = "/par"
	AuthorizationEndpoint              EndpointPath = "/authorize"
	AuthorizationCallbackEndpoint      EndpointPath = "/authorize/:callback"
	TokenEndpoint                      EndpointPath = "/token"
	UserInfoEndpoint                   EndpointPath = "/userinfo"
)
