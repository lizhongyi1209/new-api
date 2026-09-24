package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PricingValues is one model's configuration, keyed by the existing option
// names. A missing key inherits the engine's default; an explicit zero is free.
type PricingValues map[string]any

type ModelPricingChange struct {
	ModelName       string        `json:"model_name"`
	ExpectedVersion string        `json:"expected_version"`
	Pricing         PricingValues `json:"pricing"`
	Reset           bool          `json:"reset,omitempty"`
}

type ModelPricingEntry struct {
	ModelName      string                               `json:"model_name"`
	Version        string                               `json:"version"`
	Configured     PricingValues                        `json:"configured"`
	Effective      PricingValues                        `json:"effective"`
	UsageSchema    map[string]jsplugin.UsageFieldSchema `json:"usage_schema,omitempty"`
	UsageExamples  []jsplugin.UsageExample              `json:"usage_examples,omitempty"`
	UsageVariants  []PricingPluginVariant               `json:"usage_variants,omitempty"`
	PluginVariants []ModelPricingPluginVariant          `json:"plugin_variants,omitempty"`
}

type ModelPricingPluginVariant struct {
	PluginKey     string                               `json:"plugin_key"`
	PluginName    string                               `json:"plugin_name"`
	Icon          string                               `json:"icon,omitempty"`
	UsageSchema   map[string]jsplugin.UsageFieldSchema `json:"usage_schema"`
	UsageExamples []jsplugin.UsageExample              `json:"usage_examples,omitempty"`
	Configured    string                               `json:"configured"`
	Effective     string                               `json:"effective"`
	Compatible    bool                                 `json:"compatible"`
	Stale         bool                                 `json:"stale,omitempty"`
}

type ModelPricingSnapshot struct {
	Entries      []ModelPricingEntry `json:"entries"`
	Options      map[string]string   `json:"options"`
	EmptyVersion string              `json:"empty_version"`
}

var ErrModelPricingConflict = errors.New("model pricing changed; reload before saving")

// Lock order is stable across instances. Creating missing option rows inside
// the transaction also serializes the first write to an unconfigured database.
var modelPricingOptionKeys = []string{
	"AudioCompletionRatio", "AudioRatio", "CacheRatio", "CompletionRatio",
	"CreateCacheRatio", "ImageRatio", "ModelPrice", "ModelRatio", "VideoCompletionRatio",
	"billing_setting.billing_expr", "billing_setting.billing_mode",
	billing_setting.PluginBillingExprOption,
}

var modelPricingMutationMu sync.Mutex

func IsModelPricingOption(key string) bool {
	for _, candidate := range modelPricingOptionKeys {
		if key == candidate {
			return true
		}
	}
	return false
}

func ModelPricingVersion(values PricingValues) string {
	encoded, _ := common.Marshal(values)
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}

func defaultPricingMaps() map[string]map[string]any {
	result := make(map[string]map[string]any, len(modelPricingOptionKeys))
	for _, key := range modelPricingOptionKeys {
		result[key] = make(map[string]any)
	}
	for key, values := range ratio_setting.GetDefaultPricingMaps() {
		for name, value := range values {
			result[key][name] = value
		}
	}
	return result
}

func readModelPricingMaps(db *gorm.DB) (map[string]map[string]any, map[string]bool, []string, error) {
	var rows []Option
	if err := db.Where(commonKeyCol+" IN ?", modelPricingOptionKeys).Find(&rows).Error; err != nil {
		return nil, nil, nil, err
	}
	values := defaultPricingMaps()
	existing := make(map[string]bool)
	counts := make(map[string]int)
	for _, row := range rows {
		var entries map[string]any
		if err := common.UnmarshalJsonStr(row.Value, &entries); err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", row.Key, err)
		}
		if entries == nil {
			return nil, nil, nil, fmt.Errorf("%s must be a JSON object", row.Key)
		}
		values[row.Key] = entries
		existing[row.Key] = true
		counts[row.Key]++
	}
	var duplicated []string
	for _, key := range modelPricingOptionKeys {
		if counts[key] > 1 {
			duplicated = append(duplicated, key)
		}
	}
	return values, existing, duplicated, nil
}

func modelPricingValues(values map[string]map[string]any, name string) PricingValues {
	result := make(PricingValues)
	for _, key := range modelPricingOptionKeys {
		if key == billing_setting.PluginBillingExprOption {
			variants := make(map[string]any)
			for combined, expression := range values[key] {
				if plugin, modelName, ok := billing_setting.SplitPluginBillingExprKey(combined); ok && modelName == name {
					variants[plugin] = expression
				}
			}
			if len(variants) > 0 {
				result[key] = variants
			}
			continue
		}
		if value, exists := values[key][name]; exists {
			result[key] = value
		}
	}
	return result
}

func effectiveModelPricing(values map[string]map[string]any, name string) PricingValues {
	result := modelPricingValues(values, name)
	// Legacy wildcard aliases are resolved by the same normalization as relay.
	alias := ratio_setting.FormatMatchingModelName(name)
	for _, key := range []string{"ModelPrice", "ModelRatio", "CompletionRatio", "AudioRatio", "AudioCompletionRatio", "VideoCompletionRatio"} {
		delete(result, key)
		if value, exists := values[key][alias]; exists {
			result[key] = value
		}
	}
	mode, _ := result["billing_setting.billing_mode"].(string)
	if mode == "" {
		_, hasPrice := result["ModelPrice"]
		_, hasRatio := result["ModelRatio"]
		if _, builtin := billing_setting.GetBuiltinBillingExpr(name); builtin && !hasPrice && !hasRatio {
			mode = "tiered_expr"
		}
	}
	if mode == "tiered_expr" {
		result["billing_setting.billing_mode"] = mode
		if _, exists := result["billing_setting.billing_expr"]; !exists {
			if expression, ok := billing_setting.GetBuiltinBillingExpr(name); ok {
				result["billing_setting.billing_expr"] = expression
			}
		}
		return result
	}
	if _, exists := result["ModelPrice"]; exists {
		return result
	}
	if _, exists := result["ModelRatio"]; !exists && operation_setting.SelfUseModeEnabled {
		result["ModelRatio"] = float64(37.5)
	}
	// Completion ratios include engine-enforced model defaults. Expose their
	// effective value without persisting them into the editable configuration.
	var configuredCompletion *float64
	if ratio, exists := result["CompletionRatio"].(float64); exists {
		configuredCompletion = &ratio
	}
	result["CompletionRatio"] = ratio_setting.ResolveCompletionRatio(name, configuredCompletion).Ratio
	for key, fallback := range map[string]float64{
		"CacheRatio": 1, "CreateCacheRatio": 1.25, "ImageRatio": 1,
	} {
		if _, exists := result[key]; !exists {
			result[key] = fallback
		}
	}
	return result
}

// PreviewModelPricing resolves a complete draft without saving options or
// changing the in-memory pricing maps used by live requests.
func PreviewModelPricing(name string, draft PricingValues) (PricingValues, error) {
	if draft == nil {
		return nil, errors.New("pricing draft is required")
	}
	values, _, _, err := readModelPricingMaps(DB)
	if err != nil {
		return nil, err
	}
	if err := validateModelPricing(name, draft, modelPricingValues(values, name)); err != nil {
		return nil, err
	}
	for _, key := range modelPricingOptionKeys {
		if key == billing_setting.PluginBillingExprOption {
			for combined := range values[key] {
				_, modelName, ok := billing_setting.SplitPluginBillingExprKey(combined)
				if ok && modelName == name {
					delete(values[key], combined)
				}
			}
			if variants, ok := draft[key].(map[string]any); ok {
				for pluginKey, expression := range variants {
					values[key][billing_setting.PluginBillingExprKey(pluginKey, name)] = expression
				}
			}
			continue
		}
		delete(values[key], name)
		if value, exists := draft[key]; exists {
			values[key][name] = value
		}
	}
	return effectiveModelPricing(values, name), nil
}

func GetModelPricingSnapshot(names []string) (*ModelPricingSnapshot, error) {
	values, _, _, err := readModelPricingMaps(DB)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		nameSet := make(map[string]bool)
		for key, entries := range values {
			for name := range entries {
				if key == billing_setting.PluginBillingExprOption {
					_, modelName, ok := billing_setting.SplitPluginBillingExprKey(name)
					if !ok {
						continue
					}
					name = modelName
				}
				nameSet[name] = true
			}
		}
		for name := range billing_setting.GetBuiltinBillingExprCopy() {
			nameSet[name] = true
		}
		for _, plugin := range jsplugin.DefaultRegistry.Generation().Plugins() {
			for _, name := range plugin.Meta.Models {
				nameSet[name] = true
			}
		}
		for name := range nameSet {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	result := &ModelPricingSnapshot{Entries: make([]ModelPricingEntry, 0, len(names)), Options: make(map[string]string), EmptyVersion: ModelPricingVersion(PricingValues{})}
	generation := jsplugin.DefaultRegistry.Generation()
	for _, name := range names {
		configured := modelPricingValues(values, name)
		entry := ModelPricingEntry{ModelName: name, Version: ModelPricingVersion(configured), Configured: configured, Effective: effectiveModelPricing(values, name)}
		if plugin, ok := generation.GetByModel(name); ok {
			entry.UsageSchema, entry.UsageExamples = plugin.Meta.UsageForModel(name)
		} else if target, ok := ResolveTaskModelAlias(generation, name); ok {
			if plugin, ok := generation.Get(target.PluginKey); ok {
				entry.UsageSchema, entry.UsageExamples = plugin.Meta.UsageForModel(target.Declared)
			}
		}
		for _, plugin := range generation.PluginsByModel(name) {
			schema, examples := plugin.Meta.UsageForModel(name)
			if len(schema) == 0 {
				continue
			}
			variant := PricingPluginVariant{
				PluginKey: plugin.Meta.Key, PluginName: plugin.Meta.Name, Icon: plugin.Meta.Icon,
				BillingUsageSchema:   jsplugin.CloneUsageSchema(schema),
				BillingUsageExamples: jsplugin.CloneUsageExamples(examples),
			}
			variant.BillingExpr, _ = billing_setting.GetTaskPluginBillingExpr(plugin.Meta.Key, name)
			if variant.BillingExpr != "" {
				variant.BillingMode = billing_setting.BillingModeTieredExpr
			}
			variant.Version = TaskPluginPricingVersion(variant.BillingExpr)
			entry.UsageVariants = append(entry.UsageVariants, variant)
			configuredExpr := ""
			if entries, ok := configured[billing_setting.PluginBillingExprOption].(map[string]any); ok {
				configuredExpr, _ = entries[plugin.Meta.Key].(string)
			}
			effective := configuredExpr
			if effective == "" && entry.Effective["billing_setting.billing_mode"] == billing_setting.BillingModeTieredExpr {
				effective, _ = entry.Effective["billing_setting.billing_expr"].(string)
			}
			entry.PluginVariants = append(entry.PluginVariants, ModelPricingPluginVariant{
				PluginKey: plugin.Meta.Key, PluginName: plugin.Meta.Name, Icon: plugin.Meta.Icon,
				UsageSchema: schema, UsageExamples: examples, Configured: configuredExpr, Effective: effective,
				Compatible: billing_setting.TaskExprCompatible(effective, schema),
			})
			if len(entry.UsageSchema) == 0 {
				entry.UsageSchema = variant.BillingUsageSchema
				entry.UsageExamples = variant.BillingUsageExamples
			}
		}
		result.Entries = append(result.Entries, entry)
	}
	// Preserve the existing settings editor's full-map interface. Built-in
	// expressions are display defaults only; per-model writes do not persist them.
	for name, expression := range billing_setting.GetBuiltinBillingExprCopy() {
		effective := effectiveModelPricing(values, name)
		if effective["billing_setting.billing_mode"] != "tiered_expr" {
			continue
		}
		if _, ok := values["billing_setting.billing_mode"][name]; !ok {
			values["billing_setting.billing_mode"][name] = "tiered_expr"
		}
		if _, ok := values["billing_setting.billing_expr"][name]; !ok {
			values["billing_setting.billing_expr"][name] = expression
		}
	}
	for key, entries := range values {
		encoded, err := common.Marshal(entries)
		if err != nil {
			return nil, err
		}
		result.Options[key] = string(encoded)
	}
	return result, nil
}

func ValidateModelPricing(name string, values PricingValues) error {
	return validateModelPricing(name, values, nil)
}

func validateModelPricing(name string, values, previous PricingValues) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("model name is required")
	}
	generation := jsplugin.DefaultRegistry.Generation()
	previousVariants, _ := previous[billing_setting.PluginBillingExprOption].(map[string]any)
	variants, _ := values[billing_setting.PluginBillingExprOption].(map[string]any)
	for key, value := range values {
		if !IsModelPricingOption(key) {
			return fmt.Errorf("unsupported pricing field: %s", key)
		}
		if key == billing_setting.PluginBillingExprOption {
			if variants == nil {
				return errors.New("plugin billing expressions must be a plugin-to-expression object")
			}
			for pluginKey, raw := range variants {
				expression, ok := raw.(string)
				if !ok || strings.TrimSpace(expression) == "" {
					return fmt.Errorf("plugin %s billing expression is required", pluginKey)
				}
				plugin, exists := generation.Get(pluginKey)
				if !exists || !slices.Contains(plugin.Meta.Models, name) {
					if previousVariants[pluginKey] == expression {
						continue
					}
					return fmt.Errorf("plugin %s does not declare model %s", pluginKey, name)
				}
				schema, _ := plugin.Meta.UsageForModel(name)
				if err := billing_setting.SmokeTestTaskExpr(expression, schema); err != nil {
					return fmt.Errorf("plugin %s: %w", pluginKey, err)
				}
			}
			continue
		}
		if key == "billing_setting.billing_mode" {
			if value != "ratio" && value != "tiered_expr" {
				return errors.New("invalid billing mode")
			}
			continue
		}
		if key == "billing_setting.billing_expr" {
			expression, ok := value.(string)
			if !ok || strings.TrimSpace(expression) == "" {
				return errors.New("billing expression is required")
			}
			if _, err := billingexpr.CompileFromCache(expression); err != nil {
				return fmt.Errorf("model %s: %w", name, err)
			}
			var err error
			if plugins := generation.PluginsByModel(name); len(plugins) > 0 {
				for _, plugin := range plugins {
					if _, overridden := variants[plugin.Meta.Key]; overridden {
						continue
					}
					schema, _ := plugin.Meta.UsageForModel(name)
					if err = billing_setting.SmokeTestTaskExpr(expression, schema); err != nil {
						return fmt.Errorf("model %s: plugin %s: %w", name, plugin.Meta.Key, err)
					}
				}
			} else if target, resolved := ResolveTaskModelAlias(generation, name); resolved {
				if plugin, ok := generation.Get(target.PluginKey); ok {
					schema, _ := plugin.Meta.UsageForModel(target.Declared)
					err = billing_setting.SmokeTestTaskExpr(expression, schema)
				} else {
					err = billing_setting.SmokeTestExpr(expression)
				}
			} else if previous[key] != expression || len(billingexpr.UsedUsageKeys(expression)) == 0 {
				if billingexpr.UsesUsagePricing(expression) {
					return fmt.Errorf("model %s: no task plugin usage schema; usage-derived billing requires a configured usage schema", name)
				}
				err = billing_setting.SmokeTestExpr(expression)
			}
			if err != nil {
				return fmt.Errorf("model %s: %w", name, err)
			}
			continue
		}
		number, ok := value.(float64)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 {
			return fmt.Errorf("%s must be a finite, non-negative number", key)
		}
	}
	if values["billing_setting.billing_mode"] == "tiered_expr" {
		if _, exists := values["billing_setting.billing_expr"]; !exists {
			if _, builtin := billing_setting.GetBuiltinBillingExpr(name); !builtin {
				return errors.New("billing expression is required")
			}
		}
	}
	return nil
}

func UpdateModelPricing(changes []ModelPricingChange) error {
	if len(changes) == 0 {
		return errors.New("select model pricing changes before saving")
	}
	seen := make(map[string]bool)
	for _, change := range changes {
		if seen[change.ModelName] {
			return errors.New("duplicate model pricing change")
		}
		seen[change.ModelName] = true
		if change.ExpectedVersion == "" {
			return ErrModelPricingConflict
		}
	}
	return mutateModelPricingOptions(func(_ *gorm.DB, values map[string]map[string]any) error {
		defaults := defaultPricingMaps()
		for _, change := range changes {
			previous := modelPricingValues(values, change.ModelName)
			if ModelPricingVersion(previous) != change.ExpectedVersion {
				return fmt.Errorf("%w: %s", ErrModelPricingConflict, change.ModelName)
			}
			pricing := change.Pricing
			if change.Reset {
				pricing = modelPricingValues(defaults, change.ModelName)
			}
			if err := validateModelPricing(change.ModelName, pricing, previous); err != nil {
				return err
			}
			for _, key := range modelPricingOptionKeys {
				if key == billing_setting.PluginBillingExprOption {
					for combined := range values[key] {
						_, modelName, ok := billing_setting.SplitPluginBillingExprKey(combined)
						if ok && modelName == change.ModelName {
							delete(values[key], combined)
						}
					}
					if variants, ok := pricing[key].(map[string]any); ok {
						for pluginKey, expression := range variants {
							values[key][billing_setting.PluginBillingExprKey(pluginKey, change.ModelName)] = expression
						}
					}
					continue
				}
				delete(values[key], change.ModelName)
				if value, exists := pricing[key]; exists {
					values[key][change.ModelName] = value
				}
			}
		}
		return nil
	})
}

// UpdateModelPricingOptions keeps legacy single-option callers on the same
// locking, validation and transaction path as the model-level API.
func UpdateModelPricingOptions(updates map[string]string) error {
	return mutateModelPricingOptions(func(_ *gorm.DB, values map[string]map[string]any) error {
		previous := maps.Clone(values)
		names := make(map[string]bool)
		for key, raw := range updates {
			if !IsModelPricingOption(key) {
				return fmt.Errorf("unsupported pricing field: %s", key)
			}
			var entries map[string]any
			if err := common.UnmarshalJsonStr(raw, &entries); err != nil {
				return err
			}
			if entries == nil {
				return fmt.Errorf("%s must be a JSON object", key)
			}
			for name := range values[key] {
				if key == billing_setting.PluginBillingExprOption {
					_, modelName, ok := billing_setting.SplitPluginBillingExprKey(name)
					if !ok {
						return fmt.Errorf("invalid plugin billing key: %s", name)
					}
					name = modelName
				}
				names[name] = true
			}
			values[key] = entries
			for name := range entries {
				if key == billing_setting.PluginBillingExprOption {
					_, modelName, ok := billing_setting.SplitPluginBillingExprKey(name)
					if !ok {
						return fmt.Errorf("invalid plugin billing key: %s", name)
					}
					name = modelName
				}
				names[name] = true
			}
		}
		for name := range names {
			if err := validateModelPricing(name, modelPricingValues(values, name), modelPricingValues(previous, name)); err != nil {
				return err
			}
		}
		return nil
	})
}

func mutateModelPricingOptions(mutate func(*gorm.DB, map[string]map[string]any) error) error {
	modelPricingMutationMu.Lock()
	defer modelPricingMutationMu.Unlock()
	var committed map[string]map[string]any
	err := DB.Transaction(func(tx *gorm.DB) error {
		values, existing, duplicated, err := readModelPricingMaps(lockForUpdate(tx))
		if err != nil {
			return err
		}
		if len(duplicated) > 0 {
			common.SysError("options table has duplicate pricing keys [" + strings.Join(duplicated, ", ") + "]; the table is missing a primary key")
		}
		defaults := defaultPricingMaps()
		for _, key := range modelPricingOptionKeys {
			if existing[key] {
				continue
			}
			encoded, err := common.Marshal(defaults[key])
			if err != nil {
				return err
			}
			row := Option{Key: key, Value: string(encoded)}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		if err := mutate(tx, values); err != nil {
			return err
		}
		for _, key := range modelPricingOptionKeys {
			encoded, err := common.Marshal(values[key])
			if err != nil {
				return err
			}
			if err := tx.Model(&Option{}).Where(commonKeyCol+" = ?", key).Update("value", string(encoded)).Error; err != nil {
				return err
			}
		}
		committed = values
		return nil
	})
	if err != nil {
		return err
	}
	for _, key := range modelPricingOptionKeys {
		encoded, _ := common.Marshal(committed[key])
		if err := updateOptionMap(key, string(encoded)); err != nil {
			return err
		}
	}
	RefreshPricing()
	ratio_setting.InvalidateExposedDataCache()
	return nil
}
