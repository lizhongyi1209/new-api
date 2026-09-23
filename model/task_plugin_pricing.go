package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TaskPluginPricingChange struct {
	PluginKey       string `json:"plugin_key"`
	ModelName       string `json:"model_name"`
	ExpectedVersion string `json:"expected_version"`
	BillingExpr     string `json:"billing_expr"`
	Reset           bool   `json:"reset,omitempty"`
}

func TaskPluginPricingVersion(expression string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(expression)))
}

func UpdateTaskPluginPricing(change TaskPluginPricingChange) error {
	if !jsplugin.ValidPluginKey(change.PluginKey) || strings.TrimSpace(change.ModelName) != change.ModelName || change.ModelName == "" {
		return errors.New("invalid task plugin pricing target")
	}
	if change.ExpectedVersion == "" {
		return ErrModelPricingConflict
	}
	plugin, found := jsplugin.DefaultRegistry.Generation().Get(change.PluginKey)
	if !found || !slices.Contains(plugin.Meta.Models, change.ModelName) {
		return errors.New("task plugin does not serve this model")
	}
	if !change.Reset {
		schema, _ := plugin.Meta.UsageForModel(change.ModelName)
		if err := billing_setting.SmokeTestTaskExpr(change.BillingExpr, schema); err != nil {
			return fmt.Errorf("plugin %s model %s: %w", change.PluginKey, change.ModelName, err)
		}
	}

	modelPricingMutationMu.Lock()
	defer modelPricingMutationMu.Unlock()
	var committed string
	err := DB.Transaction(func(tx *gorm.DB) error {
		row := Option{Key: billing_setting.TaskPluginPricingOptionKey}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		if err := lockForUpdate(tx).Where(commonKeyCol+" = ?", billing_setting.TaskPluginPricingOptionKey).First(&row).Error; err != nil {
			return err
		}
		values := make(map[string]map[string]string)
		if row.Value != "" {
			if err := common.UnmarshalJsonStr(row.Value, &values); err != nil {
				return err
			}
			if values == nil {
				return errors.New("task plugin pricing must be a JSON object")
			}
		}
		current := values[change.PluginKey][change.ModelName]
		if TaskPluginPricingVersion(current) != change.ExpectedVersion {
			return ErrModelPricingConflict
		}
		if change.Reset {
			delete(values[change.PluginKey], change.ModelName)
			if len(values[change.PluginKey]) == 0 {
				delete(values, change.PluginKey)
			}
		} else {
			if values[change.PluginKey] == nil {
				values[change.PluginKey] = make(map[string]string)
			}
			values[change.PluginKey][change.ModelName] = change.BillingExpr
		}
		encoded, err := common.Marshal(values)
		if err != nil {
			return err
		}
		committed = string(encoded)
		return tx.Model(&Option{}).Where(commonKeyCol+" = ?", billing_setting.TaskPluginPricingOptionKey).Update("value", committed).Error
	})
	if err != nil {
		return err
	}
	if err := updateOptionMap(billing_setting.TaskPluginPricingOptionKey, committed); err != nil {
		return err
	}
	RefreshPricing()
	return nil
}
