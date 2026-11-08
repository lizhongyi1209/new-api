package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedanceAssetFamilyAndModelAliases(t *testing.T) {
	assert.Equal(t, seedanceAssetFamilyDreamina, seedanceAssetFamilyFromWorkflow(" Dreamina "))
	assert.Equal(t, seedanceAssetFamilyDreamina, seedanceAssetFamilyFromWorkflow("hc"))
	assert.Equal(t, seedanceAssetFamilyDreamina, seedanceAssetFamilyFromWorkflow("df"))
	assert.Equal(t, seedanceAssetFamilyDoubao, seedanceAssetFamilyFromWorkflow("doubao"))
	assert.Equal(t, "", seedanceAssetFamilyFromWorkflow("standard"))

	assert.Equal(t, seedanceAssetFamilyDreamina, seedanceAssetFamilyFromPath("/v2/sd-max/assets"))
	assert.Equal(t, seedanceAssetFamilyDreamina, seedanceAssetFamilyFromPath("/v2/sd-max/assets/mva-1"))
	assert.Equal(t, seedanceAssetFamilyDoubao, seedanceAssetFamilyFromPath("/v2/db-sd-max/assets/mva-1"))
	assert.Equal(t, seedanceAssetFamilyDreamina, seedanceAssetFamilyFromPath("/v1/sd/assets"))
	assert.Equal(t, seedanceAssetFamilyDreamina, seedanceAssetFamilyFromPath("/v1/sd-5/assets/asset-1"))
	assert.Equal(t, "", seedanceAssetFamilyFromPath("/v1/assets"))

	assert.Equal(t, seedanceAssetFamilyDreamina, seedanceModelFamily("dreamina-seedance-2-0"))
	assert.Equal(t, seedanceAssetFamilyDreamina, seedanceModelFamily("dreamina-seedance-2-0-260128-max"))
	assert.Equal(t, seedanceAssetFamilyDoubao, seedanceModelFamily("doubao-seedance-2-0-fast"))
	assert.Equal(t, "", seedanceModelFamily("my-video"))
	assert.Contains(t, seedanceAssetModelCandidates("dreamina-seedance-2-0-hc"), "dreamina-seedance-2-0-260128-max")
	assert.Contains(t, seedanceAssetModelCandidates("doubao-seedance-2-0-260128-max"), "doubao-seedance-2-0")

	assert.Equal(t, "dreamina-seedance-2-0-260128-max", seedanceAssetModelFromBody([]byte(`{"URL":"https://cdn.example.com/a.png","Model":" dreamina-seedance-2-0-260128-max "}`)))
	assert.Equal(t, "dreamina-seedance-2-0-hc", seedanceAssetModelFromBody([]byte(`{"model":"dreamina-seedance-2-0-hc"}`)))
}

func TestMappedSeedanceAssetModelFollowsChannelMapping(t *testing.T) {
	mapping := `{"dreamina-seedance-2-0-hc":"dreamina-seedance-2-0-260128-max","dreamina-seedance-2-0":"dreamina-seedance-2-0-260128-max"}`
	channel := &model.Channel{ModelMapping: &mapping}

	assert.Equal(t, "dreamina-seedance-2-0-260128-max", mappedSeedanceAssetModel(channel, "dreamina-seedance-2-0-hc"))
	assert.Equal(t, "dreamina-seedance-2-0-260128-max", mappedSeedanceAssetModel(channel, " dreamina-seedance-2-0 "))
	assert.Equal(t, "dreamina-seedance-2-0-260128-max", mappedSeedanceAssetModel(channel, "dreamina-seedance-2-0-260128-max"))
	assert.Equal(t, "", mappedSeedanceAssetModel(channel, " "))
}

func TestSelectSeedanceAssetChannelBindsFamilyNotHighestPriority(t *testing.T) {
	db, dialect := openTaskDialectDatabase(t, &model.Channel{})
	previousDB := model.DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	model.DB = db
	common.SetDatabaseTypes(dialect, dialect)
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMain, previousLog)
	})

	high, low, mid := int64(100), int64(10), int64(50)
	heavy := uint(5)
	dreaminaMap := `{"dreamina-seedance-2-0-hc":"dreamina-seedance-2-0-260128-max","dreamina-seedance-2-0":"dreamina-seedance-2-0-260128-max","my-video":"dreamina-seedance-2-0-260128-max"}`
	doubaoMap := `{"doubao-seedance-2-0":"doubao-seedance-2-0-260128-max","doubao-seedance-2-0-fast":"doubao-seedance-2-0-fast-260128-max"}`
	require.NoError(t, db.Create(&model.Channel{
		Id: 465, Type: constant.ChannelTypeServiceInferenceVideo, Status: common.ChannelStatusEnabled,
		Key: "doubao-key", Name: "doubao", Group: "default", Priority: &high,
		Models: "doubao-seedance-2-0,doubao-seedance-2-0-fast", ModelMapping: &doubaoMap,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id: 265, Type: constant.ChannelTypeServiceInferenceVideo, Status: common.ChannelStatusEnabled,
		Key: "dreamina-key", Name: "dreamina", Group: "default", Priority: &low,
		Models: "dreamina-seedance-2-0-hc,dreamina-seedance-2-0,dreamina-seedance-2-0-fast-hc,my-video", ModelMapping: &dreaminaMap,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id: 999, Type: constant.ChannelTypeServiceInferenceVideo, Status: common.ChannelStatusEnabled,
		Key: "other-key", Name: "unrelated", Group: "default", Priority: &high,
		Models: "gpt-4",
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id: 266, Type: constant.ChannelTypeServiceInferenceVideo, Status: 2,
		Key: "disabled-dreamina", Name: "disabled", Group: "default", Priority: &high,
		Models: "dreamina-seedance-2-0-hc", ModelMapping: &dreaminaMap,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id: 300, Type: constant.ChannelTypeServiceInferenceVideo, Status: common.ChannelStatusEnabled,
		Key: "vip-dreamina", Name: "vip-dreamina", Group: "vip", Priority: &mid, Weight: &heavy,
		Models: "dreamina-seedance-2-5-hc",
	}).Error)

	dreamina, err := selectSeedanceAssetChannel([]string{"default"}, seedanceAssetFamilyDreamina, "")
	require.NoError(t, err)
	assert.Equal(t, 265, dreamina.Id)
	assert.Equal(t, "dreamina-key", dreamina.Key)

	doubao, err := selectSeedanceAssetChannel([]string{"default"}, seedanceAssetFamilyDoubao, "")
	require.NoError(t, err)
	assert.Equal(t, 465, doubao.Id)

	byUpstream, err := selectSeedanceAssetChannel([]string{"default"}, seedanceAssetFamilyDreamina, "dreamina-seedance-2-0-260128-max")
	require.NoError(t, err)
	assert.Equal(t, 265, byUpstream.Id)

	byShorthand, err := selectSeedanceAssetChannel([]string{"default"}, seedanceAssetFamilyDoubao, "doubao-seedance-2-0-fast")
	require.NoError(t, err)
	assert.Equal(t, 465, byShorthand.Id)

	byAlias, err := selectSeedanceAssetChannel([]string{"default"}, seedanceAssetFamilyDreamina, "my-video")
	require.NoError(t, err)
	assert.Equal(t, 265, byAlias.Id)

	_, err = selectSeedanceAssetChannel([]string{"default"}, seedanceAssetFamilyDreamina, "doubao-seedance-2-0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "豆包")

	vip, err := selectSeedanceAssetChannel([]string{"vip"}, seedanceAssetFamilyDreamina, "")
	require.NoError(t, err)
	assert.Equal(t, 300, vip.Id)

	_, err = selectSeedanceAssetChannel([]string{"default"}, seedanceAssetFamilyDreamina, "dreamina-seedance-2-5-hc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "没有可用于模型")

	assert.Equal(t, "dreamina-seedance-2-0-260128-max", mappedSeedanceAssetModel(dreamina, "dreamina-seedance-2-0-hc"))
}

func TestProxySeedanceAssetUsesTheChannelForItsFamily(t *testing.T) {
	db, dialect := openTaskDialectDatabase(t, &model.Channel{})
	previousDB := model.DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	model.DB = db
	common.SetDatabaseTypes(dialect, dialect)
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMain, previousLog)
	})

	var seen []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		seen = append(seen, r.Header.Get("Authorization")+" "+r.Method+" "+r.URL.Path+" "+string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"data":{"Id":"mva-1","Ref":"asset://mva-1","Status":"Processing"}}`)
	}))
	t.Cleanup(upstream.Close)

	high, low := int64(100), int64(10)
	dreaminaMap := `{"dreamina-seedance-2-0-hc":"dreamina-seedance-2-0-260128-max"}`
	require.NoError(t, db.Create(&model.Channel{
		Id: 465, Type: constant.ChannelTypeServiceInferenceVideo, Status: common.ChannelStatusEnabled,
		Key: "doubao-key", Group: "default", Priority: &high, BaseURL: &upstream.URL,
		Models: "doubao-seedance-2-0-260128-max",
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id: 265, Type: constant.ChannelTypeServiceInferenceVideo, Status: common.ChannelStatusEnabled,
		Key: "dreamina-key", Group: "default", Priority: &low, BaseURL: &upstream.URL,
		Models: "dreamina-seedance-2-0-hc", ModelMapping: &dreaminaMap,
	}).Error)

	gin.SetMode(gin.TestMode)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(method, path, strings.NewReader(body))
		context.Request.Header.Set("Content-Type", "application/json")
		context.Set(string(constant.ContextKeyUsingGroup), "default")
		if method == http.MethodPost && strings.Contains(path, "/v1/seedance/assets") {
			CreateUnifiedSeedanceAsset(context)
			return recorder
		}
		ProxySeedanceAssetAPI(context)
		return recorder
	}

	direct := call(http.MethodPost, "/v2/sd-max/assets", `{"URL":"https://cdn.example.com/a.png","AssetType":"Image","Name":"ref","Model":"dreamina-seedance-2-0-260128-max"}`)
	require.Equal(t, http.StatusOK, direct.Code, direct.Body.String())
	unified := call(http.MethodPost, "/v1/seedance/assets", `{"type":"dreamina","url":"https://cdn.example.com/a.png","name":"ref","model":"dreamina-seedance-2-0-hc"}`)
	require.Equal(t, http.StatusOK, unified.Code, unified.Body.String())
	doubao := call(http.MethodPost, "/v2/db-sd-max/assets", `{"URL":"https://cdn.example.com/a.png","AssetType":"Image"}`)
	require.Equal(t, http.StatusOK, doubao.Code, doubao.Body.String())
	query := call(http.MethodGet, "/v2/sd-max/assets/mva-1", "")
	require.Equal(t, http.StatusOK, query.Code, query.Body.String())

	require.Len(t, seen, 4)
	assert.Equal(t, `Bearer dreamina-key POST /v2/sd-max/assets {"URL":"https://cdn.example.com/a.png","AssetType":"Image","Name":"ref","Model":"dreamina-seedance-2-0-260128-max"}`, seen[0])
	assert.Contains(t, seen[1], "Bearer dreamina-key POST /v2/sd-max/assets ")
	assert.Contains(t, seen[1], `"Model":"dreamina-seedance-2-0-260128-max"`)
	assert.Equal(t, `Bearer doubao-key POST /v2/db-sd-max/assets {"URL":"https://cdn.example.com/a.png","AssetType":"Image"}`, seen[2])
	assert.Equal(t, "Bearer dreamina-key GET /v2/sd-max/assets/mva-1 ", seen[3])
}
