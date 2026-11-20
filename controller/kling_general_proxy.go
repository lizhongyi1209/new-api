package controller

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel/task/kling"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// ProxyKlingGeneralAPI forwards voice and other official /v1/general calls.
// It keeps the /kling prefix, disables callbacks, and removes merchant billing data.
func ProxyKlingGeneralAPI(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if c.Request.Method == http.MethodPost && len(body) > 0 {
		var fields map[string]any
		if err := common.Unmarshal(body, &fields); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "expected a JSON request body"})
			return
		}
		if kling.HasCallback(fields) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "kling_callback_disabled", "message": kling.CallbackDisabledMessage})
			return
		}
	}
	channelID, _ := strconv.Atoi(c.Query("channel_id"))
	channel, key, _, err := resolveKlingChannel(channelID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	query := c.Request.URL.Query()
	query.Del("channel_id")
	path := strings.TrimPrefix(c.Request.URL.Path, "/kling")
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	resp, err := kling.NewOfficialElementClient(key, channel.GetBaseURL(), "").DoRaw(c.Request.Method, path, body)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	respBody, _ = service.RedactUpstreamCharges(respBody)
	c.Data(resp.StatusCode, contentType, respBody)
}
