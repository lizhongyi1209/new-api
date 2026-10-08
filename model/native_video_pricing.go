package model

import (
	"fmt"
	"reflect"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/videoexpr"
	"gorm.io/gorm"
)

// nativeVideoPricingCatalog reads only routing metadata, never channel keys.
// Disabled channels are included so saved historical model prices remain editable.
func nativeVideoPricingCatalog(db *gorm.DB) (map[string]videoexpr.Metadata, map[string]error, error) {
	catalog := make(map[string]videoexpr.Metadata)
	conflicts := make(map[string]error)
	if db == nil {
		return catalog, conflicts, nil
	}
	var channels []Channel
	if err := db.Select("id", "type", "models", "model_mapping").Find(&channels).Error; err != nil {
		return nil, nil, err
	}
	for _, channel := range channels {
		mapping := map[string]string{}
		if err := common.UnmarshalJsonStr(channel.GetModelMapping(), &mapping); channel.GetModelMapping() != "" && err != nil {
			for _, name := range channel.GetModels() {
				conflicts[name] = fmt.Errorf("channel %d model mapping is invalid", channel.Id)
			}
			continue
		}
		for _, name := range channel.GetModels() {
			upstream, cyclic := followChannelModelMapping(mapping, name)
			if cyclic {
				conflicts[name] = fmt.Errorf("model %s has a cyclic channel mapping", name)
				continue
			}
			meta := videoexpr.ForModel(channel.Type, upstream)
			if meta.Meter == "" {
				continue
			}
			if previous, exists := catalog[name]; exists && (previous.Meter != meta.Meter || !reflect.DeepEqual(previous.Schema, meta.Schema)) {
				conflicts[name] = fmt.Errorf("model %s has incompatible native video meters; use separate billing model names", name)
				continue
			}
			catalog[name] = meta
		}
	}
	for name := range conflicts {
		delete(catalog, name)
	}
	return catalog, conflicts, nil
}
