package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicContentRevalidation(t *testing.T) {
	originalOptions := common.OptionMap
	legal := system_setting.GetLegalSettings()
	originalAgreement, originalPrivacy := legal.UserAgreement, legal.PrivacyPolicy
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.OptionMap = originalOptions
		legal.UserAgreement, legal.PrivacyPolicy = originalAgreement, originalPrivacy
	})
	cases := []struct {
		path    string
		handler gin.HandlerFunc
		option  string
	}{
		{"/api/notice", GetNotice, "Notice"},
		{"/api/about", GetAbout, "About"},
		{"/api/home_page_content", GetHomePageContent, "HomePageContent"},
		{"/api/user-agreement", GetUserAgreement, ""},
		{"/api/privacy-policy", GetPrivacyPolicy, ""},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			common.OptionMap[tc.option] = "Public fixture"
			legal.UserAgreement, legal.PrivacyPolicy = "Public fixture", "Public fixture"
			first := httptest.NewRecorder()
			first.Header().Add("Vary", "Origin")
			ctx, _ := gin.CreateTestContext(first)
			ctx.Request = httptest.NewRequest(http.MethodGet, tc.path, nil)
			ctx.Set("id", 1)
			tc.handler(ctx)
			require.Equal(t, http.StatusOK, first.Code)
			etag := first.Header().Get("ETag")
			require.NotEmpty(t, etag)
			assert.Equal(t, "no-cache", first.Header().Get("Cache-Control"))
			assert.Contains(t, strings.Join(first.Header().Values("Vary"), ","), "Accept-Encoding")
			assert.Contains(t, first.Header().Values("Vary"), "Origin")
			var payload publicContentResponse
			require.NoError(t, common.Unmarshal(first.Body.Bytes(), &payload))
			assert.Equal(t, publicContentResponse{Success: true, Data: "Public fixture"}, payload)

			second := httptest.NewRecorder()
			ctx, _ = gin.CreateTestContext(second)
			ctx.Request = httptest.NewRequest(http.MethodGet, tc.path, nil)
			ctx.Request.Header.Set("If-None-Match", `"other", `+etag)
			ctx.Set("id", 2)
			tc.handler(ctx)
			ctx.Writer.WriteHeaderNow()
			assert.Equal(t, http.StatusNotModified, second.Code)
			assert.Empty(t, second.Body.String())
			assert.Equal(t, etag, second.Header().Get("ETag"))

			common.OptionMap[tc.option] = "Edited public fixture"
			legal.UserAgreement, legal.PrivacyPolicy = "Edited public fixture", "Edited public fixture"
			changed := httptest.NewRecorder()
			ctx, _ = gin.CreateTestContext(changed)
			ctx.Request = httptest.NewRequest(http.MethodGet, tc.path, nil)
			ctx.Request.Header.Set("If-None-Match", etag)
			tc.handler(ctx)
			require.Equal(t, http.StatusOK, changed.Code)
			assert.NotEqual(t, etag, changed.Header().Get("ETag"))
			require.NoError(t, common.Unmarshal(changed.Body.Bytes(), &payload))
			assert.Equal(t, "Edited public fixture", payload.Data)
		})
	}
}
