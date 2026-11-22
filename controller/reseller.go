package controller

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func ResellerAccessRequired(c *gin.Context) {
	allowed, err := model.CheckResellerAccess(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Reseller data is unavailable"})
		return
	}
	if !allowed {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "RESELLER_ACCESS_REQUIRED", "message": "Reseller access is required"})
		return
	}
	c.Next()
}

func GetResellerBootstrap(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	customers, err := model.GetResellerCustomers(ctx, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	models, groups, err := model.GetResellerLogOptions(ctx, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"customers": customers, "models": models, "groups": groups, "as_of": time.Now().Unix()})
}

func GetResellerPricing(c *gin.Context) {
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	usableGroups := service.GetUserUsableGroups(user.Group)
	officialRatios := ratio_setting.GetGroupRatioCopy()
	costRatios := make(map[string]float64, len(officialRatios))
	specialGroups := make(map[string]bool)
	for group, official := range officialRatios {
		costRatios[group] = official
		if special, ok := ratio_setting.GetGroupGroupRatio(user.Group, group); ok {
			costRatios[group] = special
			specialGroups[group] = true
		}
	}
	for group := range usableGroups {
		if _, ok := officialRatios[group]; !ok {
			officialRatios[group] = 1
			costRatios[group] = 1
		}
		if special, ok := ratio_setting.GetGroupGroupRatio(user.Group, group); ok {
			costRatios[group] = special
			specialGroups[group] = true
		}
	}
	catalog := model.BuildResellerPricingCatalog(
		model.GetPricing(),
		user.Group,
		usableGroups[user.Group],
		usableGroups,
		officialRatios,
		costRatios,
		specialGroups,
	)
	catalog.AsOf = time.Now().Unix()
	c.Header("Cache-Control", "private, no-store")
	common.ApiSuccess(c, catalog)
}

func resellerPeriodBounds(c *gin.Context) (int64, int64, bool) {
	now := time.Now().In(time.FixedZone("Asia/Shanghai", 8*3600))
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	end := start + 86400 - 1
	var err error
	if c.Query("from") != "" {
		start, err = strconv.ParseInt(c.Query("from"), 10, 64)
	}
	if err == nil && c.Query("to") != "" {
		end, err = strconv.ParseInt(c.Query("to"), 10, 64)
	}
	if err != nil || start <= 0 || end < start || start > now.Unix()+86400 || end > now.Unix()+86400 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid date range"})
		return 0, 0, false
	}
	return start, end, true
}

func GetResellerPeriod(c *gin.Context) {
	from, to, ok := resellerPeriodBounds(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	result, err := model.GetResellerPeriod(ctx, c.GetInt("id"), from, to, time.Now().Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetResellerLogs(c *gin.Context) {
	from, to, ok := resellerPeriodBounds(c)
	if !ok {
		return
	}
	filter := model.ResellerLogFilter{From: from, To: to, Model: c.Query("model"), Group: c.Query("group"), Token: c.Query("token"), Request: c.Query("request")}
	for key, target := range map[string]*int{"user": &filter.User, "type": &filter.Type, "page": &filter.Page} {
		if value := c.Query(key); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 || (key == "page" && parsed > 1000000) {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid filter"})
				return
			}
			*target = parsed
		}
	}
	if (filter.Type != 0 && filter.Type != 2 && filter.Type != 5 && filter.Type != 6) || len(filter.Model) > 256 || len(filter.Token) > 256 || len(filter.Group) > 64 || len(filter.Request) > 140 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid filter"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	result, err := model.GetResellerLogs(ctx, c.GetInt("id"), filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ListResellerAccess(c *gin.Context) {
	rows, err := model.ListResellerAccess(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func SetResellerAccess(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required,max=64"`
		Enabled  *bool  `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid reseller account"})
		return
	}
	if err := model.SetResellerAccess(c.Request.Context(), input.Username, *input.Enabled, c.GetInt("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Unable to change reseller access"})
		return
	}
	common.ApiSuccess(c, nil)
}
