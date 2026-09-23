package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func beginBrowserOAuthLogin(t *testing.T, provider, affiliate string, cookie *http.Cookie) (string, *http.Cookie) {
	t.Helper()
	body, err := common.Marshal(oauthStateRequest{Provider: provider, Intent: model.AuthFlowIntentLogin, Aff: affiliate})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/api/oauth/state", strings.NewReader(string(body)))
	if cookie != nil {
		request.AddCookie(cookie)
	}
	router := gin.New()
	router.POST("/api/oauth/state", GenerateOAuthCode)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			State string `json:"flow_token"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success)
	require.NotEmpty(t, result.Data.State)
	cookies := response.Result().Cookies()
	require.NotEmpty(t, cookies)
	return result.Data.State, cookies[0]
}

func TestOAuthLoginRejectsUnboundBrowserBeforeProviderExchange(t *testing.T) {
	for _, scenario := range []string{"missing cookie", "another browser", "malformed cookie", "legacy flow", "expired flow", "another browser provider error", "matching browser"} {
		t.Run(scenario, func(t *testing.T) {
			provider := setupAuthFlowControllerTest(t)
			provider.exchangeErr = errors.New("synthetic exchange failure")
			state, cookie := beginBrowserOAuthLogin(t, "auth-flow-test", "", nil)
			flow, err := model.GetAuthFlow(state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
			require.NoError(t, err)
			query := "&code=test"
			switch scenario {
			case "missing cookie":
				cookie = nil
			case "another browser", "another browser provider error":
				_, cookie = beginBrowserOAuthLogin(t, "auth-flow-test", "", nil)
				if scenario == "another browser provider error" {
					query = "&error=access_denied"
				}
			case "malformed cookie":
				cookie.Value = "invalid-nonce"
			case "legacy flow":
				require.NoError(t, model.DB.Model(flow).Update("payload", `{}`).Error)
			case "expired flow":
				require.NoError(t, model.DB.Model(flow).Update("expires_at", time.Now().Add(-time.Minute)).Error)
			}
			request := httptest.NewRequest(http.MethodGet, "/api/oauth/auth-flow-test?state="+state+query, nil)
			if cookie != nil {
				request.AddCookie(cookie)
			}
			router := gin.New()
			router.GET("/api/oauth/:provider", HandleOAuth)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if scenario == "matching browser" {
				assert.Equal(t, http.StatusInternalServerError, response.Code)
				assert.Equal(t, 1, provider.exchangeCalls)
			} else {
				assert.Equal(t, http.StatusForbidden, response.Code)
				assert.Zero(t, provider.exchangeCalls)
			}
			assert.Zero(t, provider.userInfoCalls)
			require.NoError(t, model.DB.First(flow, flow.Id).Error)
			assert.Nil(t, flow.ConsumedAt, "rejected browser or provider failures must not consume the pending login")
			var count int64
			require.NoError(t, model.DB.Model(&model.UserSession{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestOAuthBrowserCookieProtectsPendingTransactions(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(map[bool]string{false: "local HTTP", true: "secure cookie"}[secure], func(t *testing.T) {
			setupAuthFlowControllerTest(t)
			previous := common.SessionCookieSecure
			common.SessionCookieSecure = secure
			t.Cleanup(func() { common.SessionCookieSecure = previous })
			state, cookie := beginBrowserOAuthLogin(t, "auth-flow-test", "invite-code", nil)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, secure, cookie.Secure)
			assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			assert.Empty(t, cookie.Domain)
			assert.Equal(t, "/", cookie.Path)
			assert.Equal(t, 600, cookie.MaxAge)
			assert.True(t, cookie.Expires.After(time.Now()))
			if secure {
				assert.True(t, strings.HasPrefix(cookie.Name, "__Host-"))
			}
			flow, err := model.GetAuthFlow(state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
			require.NoError(t, err)
			assert.False(t, strings.Contains(flow.Payload, cookie.Value), "persist only the browser binding hash")
			var payload oauthFlowPayload
			require.NoError(t, common.UnmarshalJsonStr(flow.Payload, &payload))
			assert.Equal(t, "invite-code", payload.AffiliateCode)
			nextState, nextCookie := beginBrowserOAuthLogin(t, "auth-flow-test", "", cookie)
			assert.True(t, state != nextState, "each transaction needs its own state")
			assert.True(t, cookie.Value == nextCookie.Value, "another pending transaction must retain the same browser binding")
		})
	}
}

func TestBrowserBoundOAuthLoginCreatesSessionAndRejectsReplay(t *testing.T) {
	user, _ := setupSecurityEnrollmentTest(t)
	provider := &boundLoginOAuthProvider{userID: user.Id}
	oauth.Register("auth-flow-test", provider)
	t.Cleanup(func() { oauth.Unregister("auth-flow-test") })
	before, err := model.CountActiveUserSessions(user.Id, time.Now().Unix())
	require.NoError(t, err)
	state, cookie := beginBrowserOAuthLogin(t, "auth-flow-test", "", nil)
	router := gin.New()
	router.GET("/api/oauth/:provider", HandleOAuth)
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodGet, "/api/oauth/auth-flow-test?state="+state+"&code=test", nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if attempt == 0 {
			var result struct {
				Success bool `json:"success"`
				Data    struct {
					service.AuthBundle
					User struct {
						Id int `json:"id"`
					} `json:"user"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			require.True(t, result.Success)
			assert.Equal(t, user.Id, result.Data.User.Id)
			assert.True(t, result.Data.AccessToken != "")
		} else {
			assert.Equal(t, http.StatusForbidden, response.Code)
		}
	}
	assert.Equal(t, 1, provider.exchangeCalls)
	assert.Equal(t, 1, provider.userInfoCalls)
	after, err := model.CountActiveUserSessions(user.Id, time.Now().Unix())
	require.NoError(t, err)
	assert.Equal(t, before+1, after)
}

func TestOAuthStateOriginGuardPreventsUntrustedFlowCreation(t *testing.T) {
	for _, origin := range []string{"https://example.com", "https://attacker.example", ""} {
		t.Run(map[string]string{"https://example.com": "same origin", "https://attacker.example": "untrusted origin", "": "missing origin"}[origin], func(t *testing.T) {
			setupAuthFlowControllerTest(t)
			previous := common.SessionCookieSecure
			common.SessionCookieSecure = true
			t.Cleanup(func() { common.SessionCookieSecure = previous })
			router := gin.New()
			router.POST("/api/oauth/state", middleware.SessionCookieOriginGuard(), GenerateOAuthCode)
			request := httptest.NewRequest(http.MethodPost, "https://example.com/api/oauth/state", strings.NewReader(`{"provider":"auth-flow-test","intent":"login"}`))
			if origin != "" {
				request.Header.Set("Origin", origin)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			var flows int64
			require.NoError(t, model.DB.Model(&model.AuthFlow{}).Count(&flows).Error)
			if origin == "https://example.com" {
				assert.Equal(t, http.StatusOK, response.Code)
				assert.EqualValues(t, 1, flows)
			} else {
				assert.Equal(t, http.StatusForbidden, response.Code)
				assert.Zero(t, flows)
				assert.Empty(t, response.Result().Cookies())
			}
		})
	}
}
