package authn

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/luikyv/go-oidc/pkg/goidc"
	"github.com/stretchr/testify/assert"
)

func TestNewClientAuthnRequest(t *testing.T) {
	// Given.
	params := url.Values{}
	params.Set("client_id", "random_client_id")
	params.Set("client_secret", "random_client_secret")
	params.Set("client_assertion", "random_client_assertion")
	params.Set("client_assertion_type", "random_client_assertion_type")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// When.
	clientAuthnReq := NewClientAuthnRequest(req)

	// Then.
	assert.Equal(t, "random_client_id", clientAuthnReq.ClientID)
	assert.Equal(t, "random_client_secret", clientAuthnReq.ClientSecret)
	assert.Equal(t, "random_client_assertion", clientAuthnReq.ClientAssertion)
	assert.Equal(t, goidc.ClientAssertionType("random_client_assertion_type"), clientAuthnReq.ClientAssertionType)
}
