package authn

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"slices"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/luikyv/go-oidc/internal/oidc"
	"github.com/luikyv/go-oidc/pkg/goidc"
	"golang.org/x/crypto/bcrypt"
)

func Client(
	ctx *oidc.Context,
	req ClientAuthnRequest,
) (
	*goidc.Client,
	oidc.Error,
) {

	clientID, ok := getClientID(ctx, req)
	if !ok {
		return nil, oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid client")
	}

	client, err := ctx.Client(clientID)
	if err != nil {
		return nil, oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid client")
	}

	if err := authenticateClient(ctx, client, req); err != nil {
		return nil, err
	}

	return client, nil
}

func authenticateClient(
	ctx *oidc.Context,
	client *goidc.Client,
	req ClientAuthnRequest,
) oidc.Error {
	switch client.AuthnMethod {
	case goidc.ClientAuthnNone:
		return authenticateWithNoneAuthn(ctx, client, req)
	case goidc.ClientAuthnSecretPost:
		return authenticateWithClientSecretPost(ctx, client, req)
	case goidc.ClientAuthnSecretBasic:
		return authenticateWithClientSecretBasic(ctx, client, req)
	case goidc.ClientAuthnPrivateKeyJWT:
		return authenticateWithPrivateKeyJWT(ctx, client, req)
	case goidc.ClientAuthnSecretJWT:
		return authenticateWithClientSecretJWT(ctx, client, req)
	case goidc.ClientAuthnSelfSignedTLS:
		return authenticateWithSelfSignedTLSCertificate(ctx, client, req)
	case goidc.ClientAuthnTLS:
		return authenticateWithTLSCertificate(ctx, client, req)
	default:
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid authentication method")
	}
}

func authenticateWithNoneAuthn(
	_ *oidc.Context,
	client *goidc.Client,
	req ClientAuthnRequest,
) oidc.Error {
	if client.ID != req.ClientID {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid client")
	}
	return nil
}

func authenticateWithClientSecretPost(
	ctx *oidc.Context,
	client *goidc.Client,
	req ClientAuthnRequest,
) oidc.Error {
	if client.ID != req.ClientID || req.ClientSecret == "" {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid client")
	}
	return validateSecret(ctx, client, req.ClientSecret)
}

func authenticateWithClientSecretBasic(
	ctx *oidc.Context,
	client *goidc.Client,
	_ ClientAuthnRequest,
) oidc.Error {
	clientID, clientSecret, ok := ctx.Request().BasicAuth()
	if !ok || client.ID != clientID {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid client")
	}
	return validateSecret(ctx, client, clientSecret)
}

func validateSecret(
	_ *oidc.Context,
	client *goidc.Client,
	clientSecret string,
) oidc.Error {
	err := bcrypt.CompareHashAndPassword([]byte(client.HashedSecret), []byte(clientSecret))
	if err != nil {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid secret")
	}
	return nil
}

func authenticateWithPrivateKeyJWT(
	ctx *oidc.Context,
	client *goidc.Client,
	req ClientAuthnRequest,
) oidc.Error {

	if req.ClientAssertionType != goidc.AssertionTypeJWTBearer {
		return oidc.NewError(oidc.ErrorCodeInvalidRequest, "invalid assertion_type")
	}

	signatureAlgorithms := ctx.PrivateKeyJWTSignatureAlgorithms
	if client.AuthnSignatureAlgorithm != "" {
		signatureAlgorithms = []jose.SignatureAlgorithm{client.AuthnSignatureAlgorithm}
	}
	assertion, err := jwt.ParseSigned(req.ClientAssertion, signatureAlgorithms)
	if err != nil {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid assertion signature")
	}

	// Verify that the assertion indicates the key ID.
	if len(assertion.Headers) != 1 || assertion.Headers[0].KeyID == "" {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid kid header in the assertion")
	}

	// Verify that the key ID belongs to the client.
	jwk, err := client.PublicKey(assertion.Headers[0].KeyID)
	if err != nil {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, err.Error())
	}

	claims := jwt.Claims{}
	if err := assertion.Claims(jwk.Key, &claims); err != nil {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid assertion signature")
	}

	return areAssertionClaimsValid(ctx, client, claims, ctx.PrivateKeyJWTAssertionLifetimeSecs)
}

func authenticateWithClientSecretJWT(
	ctx *oidc.Context,
	client *goidc.Client,
	req ClientAuthnRequest,
) oidc.Error {
	if req.ClientAssertionType != goidc.AssertionTypeJWTBearer {
		return oidc.NewError(oidc.ErrorCodeInvalidRequest, "invalid assertion_type")
	}

	signatureAlgorithms := ctx.ClientSecretJWTSignatureAlgorithms
	if client.AuthnSignatureAlgorithm != "" {
		signatureAlgorithms = []jose.SignatureAlgorithm{client.AuthnSignatureAlgorithm}
	}
	assertion, err := jwt.ParseSigned(req.ClientAssertion, signatureAlgorithms)
	if err != nil {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid assertion")
	}

	claims := jwt.Claims{}
	if err := assertion.Claims([]byte(client.Secret), &claims); err != nil {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid assertion")
	}

	return areAssertionClaimsValid(ctx, client, claims, ctx.ClientSecretJWTAssertionLifetimeSecs)
}

func areAssertionClaimsValid(
	ctx *oidc.Context,
	client *goidc.Client,
	claims jwt.Claims,
	maxLifetimeSecs int64,
) oidc.Error {
	// Validate that the "iat" and "exp" claims are present and their difference is not too great.
	if claims.Expiry == nil || claims.IssuedAt == nil || int64(claims.Expiry.Time().Sub(claims.IssuedAt.Time()).Seconds()) > maxLifetimeSecs {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid time claim in the assertion")
	}

	err := claims.ValidateWithLeeway(jwt.Expected{
		Issuer:      client.ID,
		Subject:     client.ID,
		AnyAudience: ctx.Audiences(),
	}, time.Duration(0))
	if err != nil {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid assertion")
	}
	return nil
}

func authenticateWithSelfSignedTLSCertificate(
	ctx *oidc.Context,
	client *goidc.Client,
	req ClientAuthnRequest,
) oidc.Error {
	if client.ID != req.ClientID {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid client")
	}

	clientCert, ok := ctx.ClientCertificate()
	if !ok {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "client certificate not informed")
	}

	jwks, err := client.FetchPublicJWKS()
	if err != nil {
		return oidc.NewError(oidc.ErrorCodeInternalError, "could not load the client JWKS")
	}

	var jwk jose.JSONWebKey
	foundMatchingJWK := false
	for _, key := range jwks.Keys {
		if string(key.CertificateThumbprintSHA256) == hashSHA256(clientCert.Raw) ||
			string(key.CertificateThumbprintSHA1) == hashSHA1(clientCert.Raw) {
			foundMatchingJWK = true
			jwk = key
		}
	}

	if !foundMatchingJWK {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "could not find a JWK matching the client certificate")
	}

	if !comparePublicKeys(jwk.Key, clientCert.PublicKey) {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "the public key in the client certificate and ")
	}

	return nil
}

func authenticateWithTLSCertificate(
	ctx *oidc.Context,
	client *goidc.Client,
	req ClientAuthnRequest,
) oidc.Error {
	if client.ID != req.ClientID {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid client")
	}

	clientCert, ok := ctx.ClientCertificate()
	if !ok {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "client certificate not informed")
	}

	if client.TLSSubjectDistinguishedName != "" && clientCert.Subject.String() != client.TLSSubjectDistinguishedName {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid distinguished name")
	}
	if client.TLSSubjectAlternativeName != "" && !slices.Contains(clientCert.DNSNames, client.TLSSubjectAlternativeName) {
		return oidc.NewError(oidc.ErrorCodeInvalidClient, "invalid alternative name")
	}

	return nil
}

func getClientID(
	ctx *oidc.Context,
	req ClientAuthnRequest,
) (
	string,
	bool,
) {
	clientIDs := []string{}

	if req.ClientID != "" {
		clientIDs = append(clientIDs, req.ClientID)
	}

	basicClientID, _, _ := ctx.Request().BasicAuth()
	if basicClientID != "" {
		clientIDs = append(clientIDs, basicClientID)
	}

	clientIDs, ok := appendClientIDFromAssertion(ctx, clientIDs, req)
	if !ok {
		return "", false
	}

	// All the client IDs present must be equal.
	if len(clientIDs) == 0 || !allEquals(clientIDs) {
		return "", false
	}

	return clientIDs[0], true
}

func appendClientIDFromAssertion(
	ctx *oidc.Context,
	clientIDs []string,
	req ClientAuthnRequest,
) (
	[]string,
	bool,
) {
	if req.ClientAssertion == "" {
		return clientIDs, true
	}

	assertionClientID, ok := getClientIDFromAssertion(ctx, req.ClientAssertion)
	if !ok {
		return []string{}, false
	}

	return append(clientIDs, assertionClientID), true
}

func getClientIDFromAssertion(
	ctx *oidc.Context,
	assertion string,
) (
	string,
	bool,
) {
	parsedAssertion, err := jwt.ParseSigned(assertion, ctx.ClientSignatureAlgorithms())
	if err != nil {
		return "", false
	}

	var claims map[string]any
	if err := parsedAssertion.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return "", false
	}

	// The issuer claim is supposed to have the client ID.
	clientID, ok := claims[goidc.ClaimIssuer]
	if !ok {
		return "", false
	}

	clientIDAsString, ok := clientID.(string)
	if !ok {
		return "", false
	}

	return clientIDAsString, true
}

// Return true only if all the elements in values are equal.
func allEquals[T comparable](values []T) bool {
	if len(values) == 0 {
		return true
	}

	return all(
		values,
		func(value T) bool {
			return value == values[0]
		},
	)
}

// Return true if all the elements in the slice respect the condition.
func all[T interface{}](slice []T, condition func(T) bool) bool {
	for _, element := range slice {
		if !condition(element) {
			return false
		}
	}

	return true
}

func comparePublicKeys(k1 any, k2 any) bool {
	key2, ok := k2.(crypto.PublicKey)
	if !ok {
		return false
	}

	switch key1 := k1.(type) {
	case ed25519.PublicKey:
		return key1.Equal(key2)
	case *ecdsa.PublicKey:
		return key1.Equal(key2)
	case *rsa.PublicKey:
		return key1.Equal(key2)
	default:
		return false
	}
}

func hashSHA256(s []byte) string {
	hash := sha256.New()
	hash.Write([]byte(s))
	return string(hash.Sum(nil))
}

func hashSHA1(s []byte) string {
	hash := sha1.New()
	hash.Write([]byte(s))
	return string(hash.Sum(nil))
}
