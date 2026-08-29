package controller

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPrepareAsyncBillingPreservesChannelOtherSettings(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousDatabaseType := common.MainDatabaseType()
	previousCalculatePrice := service.CalculatePriceFunc

	common.MemoryCacheEnabled = false
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	service.CalculatePriceFunc = func(_ *gin.Context, _ *relaycommon.RelayInfo) (types.PriceData, error) {
		return types.PriceData{FreeModel: true}, nil
	}
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.SetMainDatabaseType(previousDatabaseType)
		service.CalculatePriceFunc = previousCalculatePrice
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	channel := model.Channel{
		Id:     901,
		Type:   constant.ChannelTypeGemini,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "file-data-capability",
		Models: "test-image-model",
		Group:  "test",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		GeminiFileDataEnabled: true,
		ImageOutputStrategy:   dto.ImageOutputStrategyLocalTempCF,
	})
	require.NoError(t, db.Create(&channel).Error)

	context, _ := gin.CreateTestContext(nil)
	relayInfo, _, apiErr := prepareAsyncBilling(context, 1, "test", channel.Id, 1, "test-image-model")
	require.Nil(t, apiErr)
	require.NotNil(t, relayInfo)
	assert.True(t, relayInfo.ChannelOtherSettings.GeminiFileDataEnabled)
	assert.Equal(t, dto.ImageOutputStrategyLocalTempCF, relayInfo.ChannelOtherSettings.ImageOutputStrategy)
}
