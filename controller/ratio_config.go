package controller

import (
	"maps"
	"net/http"

	"github.com/QuantumNous/new-api/model"

	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetRatioConfig(c *gin.Context) {
	if !ratio_setting.IsExposeRatioEnabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "倍率配置接口未启用",
		})
		return
	}

	// The public synchronization endpoint can export ordinary expressions.
	// These model names have a private settlement contract and publish retail
	// prices through the customer catalog instead.
	data := billing_setting.GetPricingSyncData(map[string]any(ratio_setting.GetExposedData()))
	for field, value := range data {
		switch values := value.(type) {
		case map[string]string:
			filtered := maps.Clone(values)
			maps.DeleteFunc(filtered, func(name string, _ string) bool { return model.HasPrivateVideoPricing(name) })
			data[field] = filtered
		case map[string]float64:
			filtered := maps.Clone(values)
			maps.DeleteFunc(filtered, func(name string, _ float64) bool { return model.HasPrivateVideoPricing(name) })
			data[field] = filtered
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}
