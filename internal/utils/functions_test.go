package utils_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/luikymagno/goidc/internal/utils"
	"github.com/luikymagno/goidc/pkg/goidc"
)

func TestExtractJARFromRequestObject_SignedRequestObjectHappyPath(t *testing.T) {
	// When.
	privateJWK := utils.GetTestPrivateRS256JWK("client_key_id")
	ctx := utils.GetTestInMemoryContext()
	ctx.JARIsEnabled = true
	ctx.JARSignatureAlgorithms = []jose.SignatureAlgorithm{jose.SignatureAlgorithm(privateJWK.GetAlgorithm())}
	ctx.JARLifetimeSecs = 60
	client := goidc.Client{
		ClientMetaInfo: goidc.ClientMetaInfo{
			PublicJWKS: &goidc.JSONWebKeySet{
				Keys: []goidc.JSONWebKey{privateJWK.GetPublic()},
			},
		},
	}

	createdAtTimestamp := goidc.GetTimestampNow()
	signer, _ := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.SignatureAlgorithm(privateJWK.GetAlgorithm()), Key: privateJWK.GetKey()},
		(&jose.SignerOptions{}).WithType("jwt").WithHeader("kid", privateJWK.GetKeyID()),
	)
	claims := map[string]any{
		string(goidc.IssuerClaim):   client.ID,
		string(goidc.AudienceClaim): ctx.Host,
		string(goidc.IssuedAtClaim): createdAtTimestamp,
		string(goidc.ExpiryClaim):   createdAtTimestamp + ctx.JARLifetimeSecs - 1,
		"client_id":                 client.ID,
		"redirect_uri":              "https://example.com",
		"response_type":             goidc.CodeResponse,
		"scope":                     "scope scope2",
		"max_age":                   600,
		"acr_values":                "0 1",
		"claims": map[string]any{
			"userinfo": map[string]any{
				"acr": map[string]any{
					"value": "0",
				},
			},
		},
	}
	request, _ := jwt.Signed(signer).Claims(claims).Serialize()

	// Then.
	jar, err := utils.ExtractJARFromRequestObject(ctx, request, client)

	// Assert.
	if err != nil {
		t.Errorf("error extracting JAR. Error: %s", err.Error())
		return
	}

	if jar.ClientID != client.ID {
		t.Errorf("Invalid JAR client_id. JAR: %v", jar)
		return
	}

	if jar.ResponseType != goidc.CodeResponse {
		t.Errorf("Invalid JAR response_type. JAR: %v", jar)
		return
	}
}

func TestValidateDPOPJWT(t *testing.T) {

	var testCases = []struct {
		Name           string
		DPOPJWT        string
		ExpectedClaims utils.DPOPJWTValidationOptions
		Context        utils.Context
		ShouldBeValid  bool
	}{
		{
			"valid_dpop_jwt",
			"eyJ0eXAiOiJkcG9wK2p3dCIsImFsZyI6IkVTMjU2IiwiandrIjp7Imt0eSI6IkVDIiwieCI6Imw4dEZyaHgtMzR0VjNoUklDUkRZOXpDa0RscEJoRjQyVVFVZldWQVdCRnMiLCJ5IjoiOVZFNGpmX09rX282NHpiVFRsY3VOSmFqSG10NnY5VERWclUwQ2R2R1JEQSIsImNydiI6IlAtMjU2In19.eyJqdGkiOiItQndDM0VTYzZhY2MybFRjIiwiaHRtIjoiUE9TVCIsImh0dSI6Imh0dHBzOi8vc2VydmVyLmV4YW1wbGUuY29tL3Rva2VuIiwiaWF0IjoxNTYyMjY1Mjk2fQ.pAqut2IRDm_De6PR93SYmGBPXpwrAk90e8cP2hjiaG5QsGSuKDYW7_X620BxqhvYC8ynrrvZLTk41mSRroapUA",
			utils.DPOPJWTValidationOptions{},
			utils.Context{
				Configuration: utils.Configuration{
					Host:                    "https://server.example.com",
					DPOPIsEnabled:           true,
					DPOPSignatureAlgorithms: []jose.SignatureAlgorithm{jose.RS256, jose.PS256, jose.ES256},
					DPOPLifetimeSecs:        99999999999,
				},
				Request: &http.Request{
					Method: http.MethodPost,
				},
				Logger: slog.Default(),
			},
			true,
		},
		{
			"valid_dpop_jwt_with_ath",
			"eyJ0eXAiOiJkcG9wK2p3dCIsImFsZyI6IkVTMjU2IiwiandrIjp7Imt0eSI6IkVDIiwieCI6Imw4dEZyaHgtMzR0VjNoUklDUkRZOXpDa0RscEJoRjQyVVFVZldWQVdCRnMiLCJ5IjoiOVZFNGpmX09rX282NHpiVFRsY3VOSmFqSG10NnY5VERWclUwQ2R2R1JEQSIsImNydiI6IlAtMjU2In19.eyJqdGkiOiJlMWozVl9iS2ljOC1MQUVCIiwiaHRtIjoiR0VUIiwiaHR1IjoiaHR0cHM6Ly9yZXNvdXJjZS5leGFtcGxlLm9yZy9wcm90ZWN0ZWRyZXNvdXJjZSIsImlhdCI6MTU2MjI2MjYxOCwiYXRoIjoiZlVIeU8ycjJaM0RaNTNFc05yV0JiMHhXWG9hTnk1OUlpS0NBcWtzbVFFbyJ9.2oW9RP35yRqzhrtNP86L-Ey71EOptxRimPPToA1plemAgR6pxHF8y6-yqyVnmcw6Fy1dqd-jfxSYoMxhAJpLjA",
			utils.DPOPJWTValidationOptions{
				AccessToken: "Kz~8mXK1EalYznwH-LC-1fBAo.4Ljp~zsPE_NeO.gxU",
			},
			utils.Context{
				Configuration: utils.Configuration{
					Host:                    "https://resource.example.org/protectedresource",
					DPOPIsEnabled:           true,
					DPOPSignatureAlgorithms: []jose.SignatureAlgorithm{jose.RS256, jose.PS256, jose.ES256},
					DPOPLifetimeSecs:        99999999999,
				},
				Request: &http.Request{
					Method: http.MethodGet,
				},
				Logger: slog.Default(),
			},
			true,
		},
	}

	for _, testCase := range testCases {
		t.Run(
			testCase.Name,
			func(t *testing.T) {
				// When.
				// ctx.Request.Method = testCase.ExpectedClaims

				// Then.
				err := utils.ValidateDPOPJWT(testCase.Context, testCase.DPOPJWT, testCase.ExpectedClaims)

				// Assert.
				isValid := err == nil
				if isValid != testCase.ShouldBeValid {
					t.Errorf("expected: %v - actual: %v - error: %s", testCase.ShouldBeValid, isValid, err)
					return
				}
			},
		)
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	refreshToken := utils.GenerateRefreshToken()
	if len(refreshToken) != goidc.RefreshTokenLength {
		t.Errorf("refresh token: %s has not %d characters", refreshToken, goidc.RefreshTokenLength)
	}
}

func TestGetURLWithQueryParams(t *testing.T) {
	testCases := []struct {
		URL                      string
		params                   map[string]string
		ExpectedParameterizedURL string
	}{
		{"http://example", map[string]string{"param1": "value1"}, "http://example?param1=value1"},
		{"http://example?param=value", map[string]string{"param1": "value1"}, "http://example?param=value&param1=value1"},
		{"http://example", map[string]string{"param1": "value1", "param2": "value2"}, "http://example?param1=value1&param2=value2"},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case %v", i), func(t *testing.T) {
			parameterizedURL := utils.GetURLWithQueryParams(testCase.URL, testCase.params)

			if parameterizedURL != testCase.ExpectedParameterizedURL {
				t.Errorf("%s is different from %s", parameterizedURL, testCase.ExpectedParameterizedURL)
			}
		})
	}

}

func TestGetURLWithFragmentParams(t *testing.T) {
	testCases := []struct {
		URL                      string
		params                   map[string]string
		ExpectedParameterizedURL string
	}{
		{"http://example", map[string]string{"param1": "value1"}, "http://example#param1=value1"},
		{"http://example#param=value", map[string]string{"param1": "value1"}, "http://example#param=value&param1=value1"},
		{"http://example", map[string]string{"param1": "value1", "param2": "value2"}, "http://example#param1=value1&param2=value2"},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case %v", i), func(t *testing.T) {
			parameterizedURL := utils.GetURLWithFragmentParams(testCase.URL, testCase.params)

			if parameterizedURL != testCase.ExpectedParameterizedURL {
				t.Errorf("%s is different from %s", parameterizedURL, testCase.ExpectedParameterizedURL)
			}
		})
	}

}

func TestGetURLWithoutParams(t *testing.T) {
	testCases := []struct {
		url         string
		expectedURL string
	}{
		{"http://example#param1=value1", "http://example"},
		{"http://example#param=value&param1=value1", "http://example"},
		{"http://example#param1=value1&param2=value2", "http://example"},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case %v", i), func(t *testing.T) {
			urlWithoutParams, err := utils.GetURLWithoutParams(testCase.url)

			if err != nil || urlWithoutParams != testCase.expectedURL {
				t.Errorf("%s is different from %s", urlWithoutParams, testCase.expectedURL)
			}
		})
	}

}

func TestIsPkceValid(t *testing.T) {
	testCases := []struct {
		codeVerifier        string
		codeChallenge       string
		codeChallengeMethod goidc.CodeChallengeMethod
		isValid             bool
	}{
		{"4ea55634198fb6a0c120d46b26359cf50ccea86fd03302b9bca9fa98", "ZObPYv2iA-CObk06I1Z0q5zWRG7gbGjZEWLX5ZC6rjQ", goidc.SHA256CodeChallengeMethod, true},
		{"42d92ec716da149b8c0a553d5cbbdc5fd474625cdffe7335d643105b", "yQ0Wg2MXS83nBOaS3yit-n-xEaEw5LQ8TlhtX_2NkLw", goidc.SHA256CodeChallengeMethod, true},
		{"179de59c7146cbb47757e7bc796c9b21d4a2be62535c4f577566816a", "ZObPYv2iA-CObk06I1Z0q5zWRG7gbGjZEWLX5ZC6rjQ", goidc.SHA256CodeChallengeMethod, false},
		{"179de59c7146cbb47757e7bc796c9b21d4a2be62535c4f577566816a", "179de59c7146cbb47757e7bc796c9b21d4a2be62535c4f577566816a", goidc.SHA256CodeChallengeMethod, false},
		{"", "ZObPYv2iA-CObk06I1Z0q5zWRG7gbGjZEWLX5ZC6rjQ", goidc.SHA256CodeChallengeMethod, false},
		{"179de59c7146cbb47757e7bc796c9b21d4a2be62535c4f577566816a", "", goidc.SHA256CodeChallengeMethod, false},
		{"random_string", "random_string", goidc.PlainCodeChallengeMethod, true},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case %v", i), func(t *testing.T) {
			isValid := utils.IsPkceValid(testCase.codeVerifier, testCase.codeChallenge, testCase.codeChallengeMethod)
			if testCase.isValid != isValid {
				t.Error("error validating PKCE")
			}
		})
	}
}

func TestAll(t *testing.T) {
	ok := utils.All([]string{"a", "b", "c"}, func(s string) bool {
		return s == "b"
	})
	if ok {
		t.Errorf("not all the elements respect the condition")
		return
	}

	ok = utils.All([]int{1, 2, 3}, func(i int) bool {
		return i > 0
	})
	if !ok {
		t.Errorf("all the elements respect the condition")
		return
	}

	ok = utils.All([]int{1, 2, 3}, func(i int) bool {
		return i == 4
	})
	if ok {
		t.Errorf("not all the elements respect the condition")
		return
	}
}

func TestAllEquals(t *testing.T) {
	testCases := []struct {
		values           []string
		allShouldBeEqual bool
	}{
		{[]string{"id1", "id1", "id1"}, true},
		{[]string{"id1"}, true},
		{[]string{}, true},
		{[]string{"id1", "id1", "id2"}, false},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case %v", i+1), func(t *testing.T) {
			if utils.AllEquals(testCase.values) != testCase.allShouldBeEqual {
				t.Error(testCase)
			}
		})
	}
}

func TestGenerateJWKThumbprint(t *testing.T) {
	dpopSigningAlgorithms := []jose.SignatureAlgorithm{jose.ES256}
	testCases := []struct {
		DPOPJWT            string
		ExpectedThumbprint string
	}{
		{
			"eyJ0eXAiOiJkcG9wK2p3dCIsImFsZyI6IkVTMjU2IiwiandrIjp7Imt0eSI6IkVDIiwieCI6Imw4dEZyaHgtMzR0VjNoUklDUkRZOXpDa0RscEJoRjQyVVFVZldWQVdCRnMiLCJ5IjoiOVZFNGpmX09rX282NHpiVFRsY3VOSmFqSG10NnY5VERWclUwQ2R2R1JEQSIsImNydiI6IlAtMjU2In19.eyJqdGkiOiItQndDM0VTYzZhY2MybFRjIiwiaHRtIjoiUE9TVCIsImh0dSI6Imh0dHBzOi8vc2VydmVyLmV4YW1wbGUuY29tL3Rva2VuIiwiaWF0IjoxNTYyMjY1Mjk2fQ.pAqut2IRDm_De6PR93SYmGBPXpwrAk90e8cP2hjiaG5QsGSuKDYW7_X620BxqhvYC8ynrrvZLTk41mSRroapUA",
			"0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I",
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case %v", i), func(t *testing.T) {
			jkt := utils.GenerateJWKThumbprint(testCase.DPOPJWT, dpopSigningAlgorithms)
			if jkt != testCase.ExpectedThumbprint {
				t.Errorf("invalid thumbprint. expected: %s - actual: %s", testCase.ExpectedThumbprint, jkt)
			}
		})
	}
}

func TestIsJWS(t *testing.T) {
	testCases := []struct {
		jws         string
		shouldBeJWS bool
	}{
		{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", true},
		{"not a jwt", false},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case %v", i+1), func(t *testing.T) {
			if utils.IsJWS(testCase.jws) != testCase.shouldBeJWS {
				t.Error(testCase)
			}
		})
	}
}

func TestIsJWE(t *testing.T) {
	testCases := []struct {
		jwe         string
		shouldBeJWE bool
	}{
		{"eyJhbGciOiJSU0EtT0FFUCIsImVuYyI6IkEyNTZHQ00ifQ.OKOawDo13gRp2ojaHV7LFpZcgV7T6DVZKTyKOMTYUmKoTCVJRgckCL9kiMT03JGeipsEdY3mx_etLbbWSrFr05kLzcSr4qKAq7YN7e9jwQRb23nfa6c9d-StnImGyFDbSv04uVuxIp5Zms1gNxKKK2Da14B8S4rzVRltdYwam_lDp5XnZAYpQdb76FdIKLaVmqgfwX7XWRxv2322i-vDxRfqNzo_tETKzpVLzfiwQyeyPGLBIO56YJ7eObdv0je81860ppamavo35UgoRdbYaBcoh9QcfylQr66oc6vFWXRcZ_ZT2LawVCWTIy3brGPi6UklfCpIMfIjf7iGdXKHzg.48V1_ALb6US04U3b.5eym8TW_c8SuK0ltJ3rpYIzOeDQz7TALvtu6UG9oMo4vpzs9tX_EFShS8iB7j6jiSdiwkIr3ajwQzaBtQD_.XFBoMYUZodetZdvTiFvSkQ", true},
		{"not a jwt", false},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case %v", i+1), func(t *testing.T) {
			if utils.IsJWE(testCase.jwe) != testCase.shouldBeJWE {
				t.Error(testCase)
			}
		})
	}
}
