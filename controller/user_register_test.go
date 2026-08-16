package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	appI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type registerResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupRegisterControllerTest(t *testing.T) {
	t.Helper()

	setupModelListControllerTestDB(t)
	require.NoError(t, appI18n.Init())

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalQuotaForNewUser := common.QuotaForNewUser
	originalQuotaForInviter := common.QuotaForInviter
	originalQuotaForInvitee := common.QuotaForInvitee
	originalGenerateDefaultToken := constant.GenerateDefaultToken

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInviter = 0
	common.QuotaForInvitee = 0
	constant.GenerateDefaultToken = false

	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.QuotaForNewUser = originalQuotaForNewUser
		common.QuotaForInviter = originalQuotaForInviter
		common.QuotaForInvitee = originalQuotaForInvitee
		constant.GenerateDefaultToken = originalGenerateDefaultToken
	})
}

func performRegisterRequest(t *testing.T, body string) registerResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")
	Register(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response registerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestRegisterRequiresValidInvitationCode(t *testing.T) {
	setupRegisterControllerTest(t)

	tests := []struct {
		name            string
		invitationCode  string
		expectedMessage string
	}{
		{
			name:            "missing invitation code",
			invitationCode:  "",
			expectedMessage: appI18n.Translate(appI18n.LangEn, appI18n.MsgUserAffCodeRequired),
		},
		{
			name:            "unknown invitation code",
			invitationCode:  "unknown",
			expectedMessage: appI18n.Translate(appI18n.LangEn, appI18n.MsgUserAffCodeInvalid),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := `{"username":"new-user","password":"password123","aff_code":"` + test.invitationCode + `"}`
			response := performRegisterRequest(t, body)

			assert.False(t, response.Success)
			assert.Equal(t, test.expectedMessage, response.Message)
			var count int64
			require.NoError(t, model.DB.Model(&model.User{}).Where("username = ?", "new-user").Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestRegisterStoresInviterFromInvitationCode(t *testing.T) {
	setupRegisterControllerTest(t)

	inviter := model.User{
		Username:    "inviter",
		Password:    "password123",
		DisplayName: "inviter",
		AffCode:     "joinme",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&inviter).Error)

	response := performRegisterRequest(t, `{"username":"new-user","password":"password123","aff_code":" joinme "}`)
	assert.True(t, response.Success)
	assert.Empty(t, response.Message)

	var registeredUser model.User
	require.NoError(t, model.DB.Where("username = ?", "new-user").First(&registeredUser).Error)
	assert.Equal(t, inviter.Id, registeredUser.InviterId)
}
