package utils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"hash"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/luikymagno/goidc/pkg/goidc"
)

type ResultChannel struct {
	Result any
	Err    goidc.OAuthError
}

func ExtractJARFromRequestObject(
	ctx OAuthContext,
	reqObject string,
	client goidc.Client,
) (
	AuthorizationRequest,
	goidc.OAuthError,
) {
	if ctx.JAREncryptionIsEnabled && IsJWE(reqObject) {
		signedReqObject, err := extractSignedRequestObjectFromEncryptedRequestObject(ctx, reqObject, client)
		if err != nil {
			return AuthorizationRequest{}, err
		}
		reqObject = signedReqObject
	}

	if !IsJWS(reqObject) {
		return AuthorizationRequest{}, goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "the request object is not a JWS")
	}

	return extractJARFromSignedRequestObject(ctx, reqObject, client)
}

func extractSignedRequestObjectFromEncryptedRequestObject(
	ctx OAuthContext,
	reqObject string,
	_ goidc.Client,
) (
	string,
	goidc.OAuthError,
) {
	encryptedReqObject, err := jose.ParseEncrypted(reqObject, ctx.JARKeyEncryptionAlgorithms(), ctx.JARContentEncryptionAlgorithms)
	if err != nil {
		return "", goidc.NewOAuthError(goidc.ErrorCodeInvalidResquestObject, "could not parse the encrypted request object")
	}

	keyID := encryptedReqObject.Header.KeyID
	if keyID == "" {
		return "", goidc.NewOAuthError(goidc.ErrorCodeInvalidResquestObject, "invalid JWE key ID")
	}

	jwk, ok := ctx.PrivateKey(keyID)
	if !ok || jwk.Usage() != string(goidc.KeyUsageEncryption) {
		return "", goidc.NewOAuthError(goidc.ErrorCodeInvalidResquestObject, "invalid JWK used for encryption")
	}

	decryptedReqObject, err := encryptedReqObject.Decrypt(jwk.Key())
	if err != nil {
		return "", goidc.NewOAuthError(goidc.ErrorCodeInvalidResquestObject, err.Error())
	}

	return string(decryptedReqObject), nil
}

func extractJARFromSignedRequestObject(
	ctx OAuthContext,
	reqObject string,
	client goidc.Client,
) (
	AuthorizationRequest,
	goidc.OAuthError,
) {
	jarAlgorithms := ctx.JARSignatureAlgorithms
	if client.JARSignatureAlgorithm != "" {
		jarAlgorithms = []jose.SignatureAlgorithm{client.JARSignatureAlgorithm}
	}
	parsedToken, err := jwt.ParseSigned(reqObject, jarAlgorithms)
	if err != nil {
		return AuthorizationRequest{}, goidc.NewOAuthError(goidc.ErrorCodeInvalidResquestObject, err.Error())
	}

	// Verify that the assertion indicates the key ID.
	if len(parsedToken.Headers) != 1 || parsedToken.Headers[0].KeyID == "" {
		return AuthorizationRequest{}, goidc.NewOAuthError(goidc.ErrorCodeInvalidResquestObject, "invalid kid header")
	}

	// Verify that the key ID belongs to the client.
	jwk, oauthErr := client.GetJWK(parsedToken.Headers[0].KeyID)
	if oauthErr != nil {
		return AuthorizationRequest{}, goidc.NewOAuthError(goidc.ErrorCodeInvalidResquestObject, oauthErr.Error())
	}

	var claims jwt.Claims
	var jarReq AuthorizationRequest
	if err := parsedToken.Claims(jwk.Key(), &claims, &jarReq); err != nil {
		return AuthorizationRequest{}, goidc.NewOAuthError(goidc.ErrorCodeInvalidResquestObject, "could not extract claims")
	}

	// Validate that the "exp" claims is present and it's not too far in the future.
	if claims.Expiry == nil || int(time.Until(claims.Expiry.Time()).Seconds()) > ctx.JARLifetimeSecs {
		return AuthorizationRequest{}, goidc.NewOAuthError(goidc.ErrorCodeInvalidResquestObject, "invalid exp claim")
	}

	err = claims.ValidateWithLeeway(jwt.Expected{
		Issuer:      client.ID,
		AnyAudience: []string{ctx.Host},
	}, time.Duration(0))
	if err != nil {
		return AuthorizationRequest{}, goidc.NewOAuthError(goidc.ErrorCodeInvalidResquestObject, "invalid claims")
	}

	return jarReq, nil
}

func ValidateDPOPJWT(
	ctx OAuthContext,
	dpopJWT string,
	expectedDPOPClaims DPOPJWTValidationOptions,
) goidc.OAuthError {
	parsedDPOPJWT, err := jwt.ParseSigned(dpopJWT, ctx.DPOPSignatureAlgorithms)
	if err != nil {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid dpop")
	}

	if len(parsedDPOPJWT.Headers) != 1 {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid dpop")
	}

	if parsedDPOPJWT.Headers[0].ExtraHeaders["typ"] != "dpop+jwt" {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid typ header. it should be dpop+jwt")
	}

	jwk := parsedDPOPJWT.Headers[0].JSONWebKey
	if jwk == nil || !jwk.Valid() || !jwk.IsPublic() {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid jwk header")
	}

	var claims jwt.Claims
	var dpopClaims DPOPJWTClaims
	if err := parsedDPOPJWT.Claims(jwk.Key, &claims, &dpopClaims); err != nil {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid dpop")
	}

	// Validate that the "iat" claim is present and it is not too far in the past.
	if claims.IssuedAt == nil || int(time.Since(claims.IssuedAt.Time()).Seconds()) > ctx.DPOPLifetimeSecs {
		return goidc.NewOAuthError(goidc.ErrorCodeUnauthorizedClient, "invalid dpop")
	}

	if claims.ID == "" {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid jti claim")
	}

	if dpopClaims.HTTPMethod != ctx.RequestMethod() {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid htm claim")
	}

	// The query and fragment components of the "htu" must be ignored.
	// Also, htu should be case-insensitive.
	httpURI, err := GetURLWithoutParams(strings.ToLower(dpopClaims.HTTPURI))
	if err != nil || !slices.Contains(ctx.Audiences(), httpURI) {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid htu claim")
	}

	if expectedDPOPClaims.AccessToken != "" && dpopClaims.AccessTokenHash != GenerateBase64URLSHA256Hash(expectedDPOPClaims.AccessToken) {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid ath claim")
	}

	if expectedDPOPClaims.JWKThumbprint != "" && GenerateJWKThumbprint(dpopJWT, ctx.DPOPSignatureAlgorithms) != expectedDPOPClaims.JWKThumbprint {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid jwk thumbprint")
	}

	err = claims.ValidateWithLeeway(jwt.Expected{}, time.Duration(0))
	if err != nil {
		return goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid dpop")
	}

	return nil
}

func GetValidTokenClaims(
	ctx OAuthContext,
	token string,
) (
	map[string]any,
	goidc.OAuthError,
) {
	parsedToken, err := jwt.ParseSigned(token, ctx.SignatureAlgorithms())
	if err != nil {
		// If the token is not a valid JWT, we'll treat it as an opaque token.
		return nil, goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "could not parse the token")
	}

	if len(parsedToken.Headers) != 1 || parsedToken.Headers[0].KeyID == "" {
		return nil, goidc.NewOAuthError(goidc.ErrorCodeInvalidRequest, "invalid header kid")
	}

	keyID := parsedToken.Headers[0].KeyID
	publicKey, ok := ctx.PublicKey(keyID)
	if !ok || publicKey.Usage() != string(goidc.KeyUsageSignature) {
		return nil, goidc.NewOAuthError(goidc.ErrorCodeAccessDenied, "invalid token")
	}

	var claims jwt.Claims
	var rawClaims map[string]any
	if err := parsedToken.Claims(publicKey.Key(), &claims, &rawClaims); err != nil {
		return nil, goidc.NewOAuthError(goidc.ErrorCodeAccessDenied, "invalid token")
	}

	if err := claims.ValidateWithLeeway(jwt.Expected{
		Issuer: ctx.Host,
	}, time.Duration(0)); err != nil {
		return nil, goidc.NewOAuthError(goidc.ErrorCodeAccessDenied, "invalid token")
	}

	return rawClaims, nil
}

func GetTokenID(ctx OAuthContext, token string) (string, goidc.OAuthError) {
	if !IsJWS(token) {
		return token, nil
	}

	claims, err := GetValidTokenClaims(ctx, token)
	if err != nil {
		return "", err
	}

	tokenID := claims[string(goidc.ClaimTokenID)]
	if tokenID == nil {
		return "", goidc.NewOAuthError(goidc.ErrorCodeAccessDenied, "invalid token")
	}

	return tokenID.(string), nil
}

func RunValidations(
	ctx OAuthContext,
	params goidc.AuthorizationParameters,
	client goidc.Client,
	validators ...func(
		ctx OAuthContext,
		params goidc.AuthorizationParameters,
		client goidc.Client,
	) goidc.OAuthError,
) goidc.OAuthError {
	for _, validator := range validators {
		if err := validator(ctx, params, client); err != nil {
			return err
		}
	}

	return nil
}

func ExtractProtectedParamsFromForm(ctx OAuthContext) map[string]any {
	protectedParams := make(map[string]any)
	for param, value := range ctx.FormData() {
		if strings.HasPrefix(param, goidc.ProtectedParamPrefix) {
			protectedParams[param] = value
		}
	}

	return protectedParams
}

func ExtractProtectedParamsFromRequestObject(ctx OAuthContext, request string) map[string]any {
	parsedRequest, err := jwt.ParseSigned(request, ctx.JARSignatureAlgorithms)
	if err != nil {
		return map[string]any{}
	}

	var claims map[string]any
	err = parsedRequest.UnsafeClaimsWithoutVerification(&claims)
	if err != nil {
		return map[string]any{}
	}

	protectedParams := make(map[string]any)
	for param, value := range claims {
		if strings.HasPrefix(param, goidc.ProtectedParamPrefix) {
			protectedParams[param] = value
		}
	}

	return protectedParams
}

func EncryptJWT(
	_ OAuthContext,
	jwtString string,
	encryptionJWK goidc.JSONWebKey,
	contentKeyEncryptionAlgorithm jose.ContentEncryption,
) (
	string,
	goidc.OAuthError,
) {
	encrypter, err := jose.NewEncrypter(
		contentKeyEncryptionAlgorithm,
		jose.Recipient{Algorithm: jose.KeyAlgorithm(encryptionJWK.Algorithm()), Key: encryptionJWK.Key(), KeyID: encryptionJWK.KeyID()},
		(&jose.EncrypterOptions{}).WithType("jwt").WithContentType("jwt"),
	)
	if err != nil {
		return "", goidc.NewOAuthError(goidc.ErrorCodeInternalError, err.Error())
	}

	encryptedUserInfoJWTJWE, err := encrypter.Encrypt([]byte(jwtString))
	if err != nil {
		return "", goidc.NewOAuthError(goidc.ErrorCodeInternalError, err.Error())
	}

	encryptedUserInfoString, err := encryptedUserInfoJWTJWE.CompactSerialize()
	if err != nil {
		return "", goidc.NewOAuthError(goidc.ErrorCodeInternalError, err.Error())
	}

	return encryptedUserInfoString, nil
}

func NewGrantSession(grantOptions goidc.GrantOptions, token Token) goidc.GrantSession {
	timestampNow := goidc.TimestampNow()
	return goidc.GrantSession{
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

func NewAuthnSession(authParams goidc.AuthorizationParameters, client goidc.Client) goidc.AuthnSession {
	return goidc.AuthnSession{
		ID:                       uuid.NewString(),
		ClientID:                 client.ID,
		AuthorizationParameters:  authParams,
		CreatedAtTimestamp:       goidc.TimestampNow(),
		Store:                    make(map[string]any),
		AdditionalTokenClaims:    make(map[string]any),
		AdditionalIDTokenClaims:  map[string]any{},
		AdditionalUserInfoClaims: map[string]any{},
	}
}

func RefreshToken() string {
	return goidc.RandomString(goidc.RefreshTokenLength, goidc.RefreshTokenLength)
}

func ClientID() string {
	return "dc-" + goidc.RandomString(goidc.DynamicClientIDLength, goidc.DynamicClientIDLength)
}

func ClientSecret() string {
	return goidc.RandomString(goidc.ClientSecretLength, goidc.ClientSecretLength)
}

func RegistrationAccessToken() string {
	return goidc.RandomString(goidc.RegistrationAccessTokenLength, goidc.RegistrationAccessTokenLength)
}

func GetURLWithQueryParams(redirectURI string, params map[string]string) string {
	parsedURL, _ := url.Parse(redirectURI)
	query := parsedURL.Query()
	for param, value := range params {
		query.Add(param, value)
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

func GetURLWithFragmentParams(redirectURI string, params map[string]string) string {
	parsedURL, _ := url.Parse(redirectURI)
	fragments, _ := url.ParseQuery(parsedURL.Fragment)
	for param, value := range params {
		fragments.Add(param, value)
	}
	parsedURL.Fragment = fragments.Encode()
	return parsedURL.String()
}

func GetURLWithoutParams(u string) (string, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String(), nil
}

func IsPkceValid(codeVerifier string, codeChallenge string, codeChallengeMethod goidc.CodeChallengeMethod) bool {
	switch codeChallengeMethod {
	case goidc.CodeChallengeMethodPlain:
		return codeChallenge == codeVerifier
	case goidc.CodeChallengeMethodSHA256:
		return codeChallenge == GenerateBase64URLSHA256Hash(codeVerifier)
	}

	return false
}

// Return true if all the elements in the slice respect the condition.
func All[T interface{}](slice []T, condition func(T) bool) bool {
	for _, element := range slice {
		if !condition(element) {
			return false
		}
	}

	return true
}

// Return true only if all the elements in values are equal.
func AllEquals[T comparable](values []T) bool {
	if len(values) == 0 {
		return true
	}

	return All(
		values,
		func(value T) bool {
			return value == values[0]
		},
	)
}

func ScopesContainsOpenID(scopes string) bool {
	return slices.Contains(goidc.SplitStringWithSpaces(scopes), goidc.ScopeOpenID.ID)
}

// Generate a JWK thumbprint for a valid DPoP JWT.
func GenerateJWKThumbprint(dpopJWT string, dpopSigningAlgorithms []jose.SignatureAlgorithm) string {
	parsedDPOPJWT, _ := jwt.ParseSigned(dpopJWT, dpopSigningAlgorithms)
	jkt, _ := parsedDPOPJWT.Headers[0].JSONWebKey.Thumbprint(crypto.SHA256)
	return base64.RawURLEncoding.EncodeToString(jkt)
}

func GenerateBase64URLSHA256Hash(s string) string {
	hash := sha256.New()
	hash.Write([]byte(s))
	return base64.RawURLEncoding.EncodeToString(hash.Sum(nil))
}

func SHA256Hash(s []byte) string {
	hash := sha256.New()
	hash.Write([]byte(s))
	return string(hash.Sum(nil))
}

func SHA1Hash(s []byte) string {
	hash := sha1.New()
	hash.Write([]byte(s))
	return string(hash.Sum(nil))
}

func HalfHashClaim(claimValue string, idTokenAlgorithm jose.SignatureAlgorithm) string {
	var hash hash.Hash
	switch idTokenAlgorithm {
	case jose.RS256, jose.ES256, jose.PS256, jose.HS256:
		hash = sha256.New()
	case jose.RS384, jose.ES384, jose.PS384, jose.HS384:
		hash = sha512.New384()
	case jose.RS512, jose.ES512, jose.PS512, jose.HS512:
		hash = sha512.New()
	default:
		hash = nil
	}

	hash.Write([]byte(claimValue))
	halfHashedClaim := hash.Sum(nil)[:hash.Size()/2]
	return base64.RawURLEncoding.EncodeToString(halfHashedClaim)
}

func IsJWS(token string) bool {
	isJWS, _ := regexp.MatchString("(^[\\w-]*\\.[\\w-]*\\.[\\w-]*$)", token)
	return isJWS
}

func IsJWE(token string) bool {
	isJWS, _ := regexp.MatchString("(^[\\w-]*\\.[\\w-]*\\.[\\w-]*\\.[\\w-]*\\.[\\w-]*$)", token)
	return isJWS
}

func ComparePublicKeys(k1 any, k2 any) bool {
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
