package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type selfLogStatResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Quota       int `json:"quota"`
		RefundQuota int `json:"refund_quota"`
	} `json:"data"`
}

func TestGetLogsSelfStatReturnsOnlyAuthenticatedUserRefundQuota(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	require.NoError(t, db.Create(&[]model.Log{
		{Username: "alice", CreatedAt: 1000, Type: model.LogTypeConsume, Quota: 1200},
		{Username: "alice", CreatedAt: 1000, Type: model.LogTypeRefund, Quota: 300},
		{Username: "bob", CreatedAt: 1000, Type: model.LogTypeRefund, Quota: 700},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("username", "alice")
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/self/stat?start_timestamp=900&end_timestamp=1100", nil)

	GetLogsSelfStat(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload selfLogStatResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, 1200, payload.Data.Quota)
	require.Equal(t, 300, payload.Data.RefundQuota)
}
