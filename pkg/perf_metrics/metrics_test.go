package perfmetrics

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummaryHourlyWindowMergesTrafficAndPreservesLegacyFields(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousPath, previousMaster := common.SQLitePath, common.IsMasterNode
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	t.Setenv("SQL_DSN", "local")
	common.SQLitePath = filepath.Join(t.TempDir(), "metrics.db")
	common.IsMasterNode = false
	require.NoError(t, model.InitDB())
	db := model.DB
	require.NoError(t, db.AutoMigrate(&model.PerfMetric{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SQLitePath, common.IsMasterNode = previousPath, previousMaster
		common.SetDatabaseTypes(previousMainType, previousLogType)
		hotBuckets.Clear()
	})
	hotBuckets.Clear()
	end := time.Date(2026, 9, 13, 0, 30, 0, 0, time.UTC).Unix()
	hour := end - end%3600
	rows := []model.PerfMetric{
		{ModelName: "weighted", Group: "default", BucketTs: hour - 23*3600, RequestCount: 1, SuccessCount: 1, TotalLatencyMs: 100},
		{ModelName: "weighted", Group: "default", BucketTs: hour - 3600, RequestCount: 2, SuccessCount: 1, TotalLatencyMs: 200},
		{ModelName: "weighted", Group: "default", BucketTs: hour - 3600 + 300, RequestCount: 8, SuccessCount: 8, TotalLatencyMs: 800},
		{ModelName: "weighted", Group: "premium", BucketTs: hour - 3600 + 600, RequestCount: 10, SuccessCount: 0},
		{ModelName: "weighted", Group: "default", BucketTs: hour, RequestCount: 2, SuccessCount: 0, TotalLatencyMs: 200},
		{ModelName: "weighted", Group: "removed", BucketTs: hour, RequestCount: 1000, SuccessCount: 1000},
		{ModelName: "weighted", Group: "default", BucketTs: hour - 25*3600, RequestCount: 1000, SuccessCount: 1000},
		{ModelName: "weighted", Group: "default", BucketTs: hour + 3600, RequestCount: 1000, SuccessCount: 1000},
	}
	require.NoError(t, db.Create(&rows).Error)
	active := &atomicBucket{}
	active.addCounters(counters{requestCount: 2, successCount: 2, totalLatencyMs: 200})
	hotBuckets.Store(bucketKey{model: "weighted", group: "default", bucketTs: hour}, active)
	hotBuckets.Store(bucketKey{model: "hidden", group: "removed", bucketTs: hour}, active)

	result, err := querySummaryAllAt(24, []string{"default", "premium"}, end)
	require.NoError(t, err)
	require.Len(t, result.Models, 1)
	assert.Equal(t, hour-23*3600, result.HourlyWindowStartTs)
	assert.Equal(t, end, result.HourlyWindowEndTs)
	summary := result.Models[0]
	assert.Equal(t, int64(25), summary.RequestCount)
	assert.Equal(t, 48.0, summary.SuccessRate)
	assert.Equal(t, []float64{100, 0, 50}, summary.RecentSuccessRates)
	require.Len(t, summary.RecentSuccessSeries, 24)
	for slot, point := range summary.RecentSuccessSeries {
		assert.Equal(t, hour-int64(23-slot)*3600, point.Ts)
	}
	require.NotNil(t, summary.RecentSuccessSeries[0].SuccessRate)
	assert.Equal(t, 100.0, *summary.RecentSuccessSeries[0].SuccessRate)
	assert.Nil(t, summary.RecentSuccessSeries[1].SuccessRate)
	require.NotNil(t, summary.RecentSuccessSeries[22].SuccessRate)
	assert.Equal(t, 45.0, *summary.RecentSuccessSeries[22].SuccessRate)
	require.NotNil(t, summary.RecentSuccessSeries[23].SuccessRate)
	assert.Equal(t, 50.0, *summary.RecentSuccessSeries[23].SuccessRate)

	// A short summary keeps its existing totals but still supplies 24-hour health.
	short, err := querySummaryAllAt(1, []string{"default", "premium"}, end)
	require.NoError(t, err)
	require.Len(t, short.Models, 1)
	assert.Equal(t, int64(4), short.Models[0].RequestCount)
	assert.Equal(t, 50.0, short.Models[0].SuccessRate)
	assert.Equal(t, summary.RecentSuccessSeries, short.Models[0].RecentSuccessSeries)

	// Advancing through midnight shifts the window without stretching old samples.
	next, err := querySummaryAllAt(24, []string{"default", "premium"}, end+3600)
	require.NoError(t, err)
	require.Len(t, next.Models, 1)
	assert.Nil(t, next.Models[0].RecentSuccessSeries[0].SuccessRate)
	assert.Equal(t, hour-22*3600, next.Models[0].RecentSuccessSeries[0].Ts)

	empty, err := querySummaryAllAt(24, []string{}, end)
	require.NoError(t, err)
	assert.Empty(t, empty.Models)
	encoded, err := common.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "request_count")
	assert.NotContains(t, string(encoded), "removed")
	assert.Contains(t, string(encoded), `"success_rate":null`)
}
