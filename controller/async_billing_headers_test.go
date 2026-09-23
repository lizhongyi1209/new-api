package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncBillingHeadersFreezeOnlyReferencedNonCredentials(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/async/v1/generateImage", nil)
	c.Request.Header.Set("X-Service-Tier", "fast")
	c.Request.Header.Set("Authorization", "synthetic-private-fixture")
	c.Request.Header.Set("Cookie", "synthetic-session-fixture")
	headers, err := freezeAsyncBillingHeaders(c, `header("X-Service-Tier") == "fast" ? tier("fast", fixed(0.04)) : tier("normal", fixed(0.02))`)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"x-service-tier": "fast"}, headers)
	c.Request.Header.Set("X-Service-Tier", "normal")
	assert.Equal(t, "fast", headers["x-service-tier"])
	for _, name := range []string{"Authorization", "Cookie", "X-Api-Key", "X-Secret", "X-Session-Id"} {
		_, err := freezeAsyncBillingHeaders(c, `true ? tier("tokens", p*2) : (header("`+name+`") == "" ? tier("request", fixed(0.04)) : tier("other", fixed(0.02)))`)
		assert.ErrorContains(t, err, "credential headers", name)
	}
	_, err = freezeAsyncBillingHeaders(c, `header(param("header_name")) == "fast" ? tier("request", fixed(0.04)) : tier("normal", fixed(0.02))`)
	assert.ErrorContains(t, err, "literal header names")
	missing, err := freezeAsyncBillingHeaders(c, `header("x-missing") == "" ? tier("request", fixed(0.04)) : tier("normal", fixed(0.02))`)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"x-missing": ""}, missing)
}
