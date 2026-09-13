package relay

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type imageReservation struct {
	held  int
	limit int
}

func (r *imageReservation) Reserve(quota int) error {
	if quota > r.limit {
		return errors.New("insufficient image quota")
	}
	if quota > r.held {
		r.held = quota
	}
	return nil
}

func (r *imageReservation) GetPreConsumedQuota() int { return r.held }
func (*imageReservation) Settle(int) error           { return nil }
func (*imageReservation) Refund(*gin.Context)        {}
func (*imageReservation) NeedsRefund() bool          { return false }

func TestImageRequestReservesFinalQuantityBeforeUpstream(t *testing.T) {
	service.InitHttpClient()
	for _, tc := range []struct {
		name, body                                       string
		tiered, passThrough, insufficient, retryToOpenAI bool
		override                                         any
		count, status                                    int
	}{
		{name: "Ali nested count", body: `{"model":"z-image","n":1,"parameters":{"n":4}}`, count: 4},
		{name: "Ali empty parameters", body: `{"model":"z-image","n":2,"parameters":{}}`, count: 2},
		{name: "legacy channel quantity override", body: `{"model":"z-image","n":1}`, override: 4, count: 4},
		{name: "expression channel quantity override", body: `{"model":"z-image","n":1}`, override: 4, count: 4, tiered: true},
		{name: "pass-through quantity", body: `{"model":"z-image","n":2,"parameters":{}}`, count: 2, passThrough: true},
		{name: "zero override rejected", body: `{"model":"z-image","n":1}`, override: 0, status: http.StatusBadRequest},
		{name: "oversized override rejected", body: `{"model":"z-image","n":1}`, override: 129, status: http.StatusBadRequest},
		{name: "insufficient reservation blocks upstream", body: `{"model":"z-image","n":1}`, override: 4, count: 4, insufficient: true, status: http.StatusForbidden},
		{name: "retry drops Ali quantity and surcharge", body: `{"model":"z-image","n":1,"parameters":{"n":4,"prompt_extend":true}}`, count: 1, retryToOpenAI: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			received := make(chan []byte, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err == nil {
					received <- body
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(w, `{"error":{"message":"fixture upstream failure","type":"upstream_error"}}`)
			}))
			t.Cleanup(upstream.Close)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			channel := constant.ChannelTypeAli
			if tc.retryToOpenAI {
				channel = constant.ChannelTypeOpenAI
			}
			common.SetContextKey(c, constant.ContextKeyChannelType, channel)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "z-image")
			common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: tc.passThrough})
			if tc.override != nil {
				common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{
					"operations": []any{map[string]any{"path": "parameters.n", "mode": "set", "value": tc.override}},
				})
			}

			request, err := helper.GetAndValidOpenAIImageRequest(c, relayconstant.RelayModeImagesGenerations)
			require.NoError(t, err)
			reservation := &imageReservation{held: 20000, limit: 500000}
			if tc.insufficient {
				reservation.limit = reservation.held
			}
			info := &relaycommon.RelayInfo{
				Request: request, OriginModelName: "z-image", RelayMode: relayconstant.RelayModeImagesGenerations,
				RequestURLPath: c.Request.URL.Path, Billing: reservation,
				PriceData: types.PriceData{UsePrice: true, ModelPrice: 0.04, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}},
			}
			if tc.tiered {
				expr := `tier("image", 40000) * image_count`
				info.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
					BillingMode: "tiered_expr", ExprString: expr, ExprHash: billingexpr.ExprHashString(expr),
					GroupRatio: 1, QuotaPerUnit: common.QuotaPerUnit, EstimatedImageCount: common.GetPointer(1),
				}
				info.BillingRequestInput = &billingexpr.RequestInput{Body: []byte(tc.body), ImageCount: common.GetPointer(1)}
				info.PriceData.UsePrice = false
			}
			if tc.retryToOpenAI {
				reservation.held = 160000
				info.PriceData.AddOtherRatio("n", 4)
				info.PriceData.AddOtherRatio("prompt_extend", 2)
				info.BillingImageCount = common.GetPointer(3)
			}

			apiErr := ImageHelper(c, info)
			require.NotNil(t, apiErr)
			if tc.status != 0 {
				assert.Equal(t, tc.status, apiErr.StatusCode)
				assert.Empty(t, received, "no upstream submission before quantity validation and reservation")
				return
			}
			assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			require.Len(t, received, 1)
			body := <-received
			path := "parameters.n"
			if tc.retryToOpenAI {
				path = "n"
			}
			assert.Equal(t, int64(tc.count), gjson.GetBytes(body, path).Int())
			assert.Equal(t, tc.count*20000, info.PriceData.QuotaToPreConsume)
			assert.GreaterOrEqual(t, reservation.held, info.PriceData.QuotaToPreConsume)
			assert.Nil(t, info.BillingImageCount)
			if tc.tiered {
				assert.Equal(t, tc.body, string(info.BillingRequestInput.Body))
				assert.Equal(t, 1, *info.BillingRequestInput.ImageCount)
			}
		})
	}
}
