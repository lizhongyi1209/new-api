package controller

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
)

type fetchModelsRequest struct {
	ChannelID      int     `json:"channel_id"`
	BaseURL        *string `json:"base_url"`
	Type           int     `json:"type"`
	Key            string  `json:"key"`
	AdvancedCustom *string `json:"advanced_custom"`
	HeaderOverride *string `json:"header_override"`
	Proxy          *string `json:"proxy"`
}

// Model discovery uses a detached draft: neither key rotation nor malformed
// settings may cause model.Channel's read helpers to save the original record.
func buildModelDiscoveryPreviewChannel(req fetchModelsRequest) (*model.Channel, error) {
	if req.ChannelID < 0 || req.Type < 0 || (req.ChannelID == 0 && req.Type == 0) {
		return nil, fmt.Errorf("invalid channel type or channel_id")
	}
	channel := &model.Channel{Type: req.Type}
	if req.ChannelID > 0 {
		saved, err := model.GetChannelById(req.ChannelID, true)
		if err != nil {
			return nil, fmt.Errorf("channel not found")
		}
		if req.Type != 0 && req.Type != saved.Type {
			return nil, fmt.Errorf("preview channel type must match the saved channel")
		}
		channel = saved
	}
	channel.Id = 0 // In particular, Codex discovery must not refresh saved credentials.
	if req.BaseURL != nil {
		channel.BaseURL = common.GetPointer(strings.TrimSpace(*req.BaseURL))
	}
	if strings.TrimSpace(req.Key) != "" {
		channel.Key = strings.TrimSpace(req.Key)
		channel.Keys = nil
		channel.ChannelInfo = model.ChannelInfo{}
	} else if channel.ChannelInfo.IsMultiKey {
		if channel.Type == constant.ChannelTypeCodex {
			return nil, fmt.Errorf("codex channel does not support multi-key model discovery")
		}
		keys := channel.GetKeys()
		start := 0
		if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
			start = channel.ChannelInfo.MultiKeyPollingIndex
			if start < 0 || start >= len(keys) {
				start = 0
			}
		}
		selectedKey := ""
		for offset := 0; offset < len(keys); offset++ {
			idx := (start + offset) % len(keys)
			status, exists := channel.ChannelInfo.MultiKeyStatusList[idx]
			if !exists || status == common.ChannelStatusEnabled {
				selectedKey = keys[idx]
				break
			}
		}
		if selectedKey == "" {
			return nil, fmt.Errorf("no enabled keys")
		}
		channel.Key = selectedKey
		channel.Keys = nil
		channel.ChannelInfo = model.ChannelInfo{}
	}
	if channel.Type != constant.ChannelTypeCodex {
		channel.Key = strings.TrimSpace(strings.Split(strings.TrimSpace(channel.Key), "\n")[0])
	}

	// Raw maps preserve local and future fields. Avoid GetSetting/GetOtherSettings:
	// those helpers repair invalid JSON by writing to the database.
	if req.AdvancedCustom != nil {
		var settings map[string]any
		if channel.OtherSettings != "" {
			if err := common.UnmarshalJsonStr(channel.OtherSettings, &settings); err != nil {
				return nil, fmt.Errorf("invalid saved channel settings")
			}
		}
		if settings == nil {
			settings = make(map[string]any)
		}
		var config map[string]any
		if err := common.UnmarshalJsonStr(strings.TrimSpace(*req.AdvancedCustom), &config); err != nil || config == nil {
			return nil, fmt.Errorf("advanced_custom must be a JSON object")
		}
		settings["advanced_custom"] = config
		data, err := common.Marshal(settings)
		if err != nil {
			return nil, fmt.Errorf("invalid channel settings")
		}
		channel.OtherSettings = string(data)
	}
	if req.HeaderOverride != nil {
		raw := strings.TrimSpace(*req.HeaderOverride)
		if raw != "" {
			var headers map[string]any
			if err := common.UnmarshalJsonStr(raw, &headers); err != nil || headers == nil {
				return nil, fmt.Errorf("header_override must be a JSON object")
			}
			for key, value := range headers {
				if relaychannel.IsHeaderPassthroughRuleKey(key) {
					continue
				}
				if _, ok := value.(string); !ok {
					return nil, fmt.Errorf("header_override values must be strings")
				}
			}
		}
		channel.HeaderOverride = &raw
	}
	if req.Proxy != nil {
		var settings map[string]any
		if channel.Setting != nil && *channel.Setting != "" {
			if err := common.UnmarshalJsonStr(*channel.Setting, &settings); err != nil {
				return nil, fmt.Errorf("invalid saved channel setting")
			}
		}
		if settings == nil {
			settings = make(map[string]any)
		}
		settings["proxy"] = strings.TrimSpace(*req.Proxy)
		data, err := common.Marshal(settings)
		if err != nil {
			return nil, fmt.Errorf("invalid channel setting")
		}
		channel.Setting = common.GetPointer(string(data))
	}
	if err := channel.ValidateSettings(); err != nil {
		// Settings can contain proxy credentials and custom headers; do not echo them.
		return nil, fmt.Errorf("invalid channel settings for model discovery")
	}
	if channel.HeaderOverride != nil && strings.TrimSpace(*channel.HeaderOverride) != "" {
		var headers map[string]any
		if err := common.UnmarshalJsonStr(*channel.HeaderOverride, &headers); err != nil || headers == nil {
			return nil, fmt.Errorf("invalid channel header override")
		}
	}
	return channel, nil
}
