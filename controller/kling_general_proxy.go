package controller

import (
	"io"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel/task/kling"

	"github.com/gin-gonic/gin"
)

// ProxyKlingGeneralAPI forwards voice and other official /v1/general calls.
// The public path keeps the /kling prefix; the body, query, and response stay official.
func ProxyKlingGeneralAPI(c *gin.Context) {
	channelID, _ := strconv.Atoi(c.Query("channel_id"))
	channel, key, _, err := resolveKlingChannel(channelID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	body, err := io.ReadAll(c.Request.Body)
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
	c.Data(resp.StatusCode, contentType, respBody)
}
