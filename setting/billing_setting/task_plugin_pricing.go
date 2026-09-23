package billing_setting

import (
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const TaskPluginPricingOptionKey = "TaskPluginPricing"

var taskPluginPricing = struct {
	sync.RWMutex
	expressions map[string]map[string]string
}{expressions: make(map[string]map[string]string)}

// Task plugin prices are scoped by plugin key and public model name. Ordinary
// model prices remain keyed only by model name.
func GetTaskPluginBillingExpr(pluginKey, modelName string) (string, bool) {
	taskPluginPricing.RLock()
	defer taskPluginPricing.RUnlock()
	expression, ok := taskPluginPricing.expressions[pluginKey][modelName]
	return expression, ok
}

func SetTaskPluginPricingFromJsonString(raw string) error {
	var expressions map[string]map[string]string
	if err := common.UnmarshalJsonStr(raw, &expressions); err != nil {
		return err
	}
	if expressions == nil {
		return fmt.Errorf("task plugin pricing must be a JSON object")
	}
	for pluginKey, models := range expressions {
		if strings.TrimSpace(pluginKey) == "" || models == nil {
			return fmt.Errorf("invalid task plugin pricing entry")
		}
		for modelName, expression := range models {
			if strings.TrimSpace(modelName) == "" || strings.TrimSpace(expression) == "" {
				return fmt.Errorf("invalid task plugin pricing expression")
			}
		}
	}
	taskPluginPricing.Lock()
	taskPluginPricing.expressions = expressions
	taskPluginPricing.Unlock()
	return nil
}
