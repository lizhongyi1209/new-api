package controller

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	seedanceAssetFamilyDreamina = "dreamina"
	seedanceAssetFamilyDoubao   = "doubao"
)

// Public names and upstream names that share one Seedance organization.
// Channel 265-style keys list the Dreamina request names; channel 465-style
// keys list the Doubao shorthand. Generation routes by those names, so asset
// pre-upload must recognize the same aliases.
var seedanceAssetAliasGroups = [][]string{
	{"dreamina-seedance-2-0-hc", "dreamina-seedance-2-0", "dreamina-seedance-2-0-260128", "dreamina-seedance-2-0-260128-max"},
	{"dreamina-seedance-2-0-fast-hc", "dreamina-seedance-2-0-fast", "dreamina-seedance-2-0-fast-260128", "dreamina-seedance-2-0-fast-260128-max"},
	{"dreamina-seedance-2-0-mini-hc", "dreamina-seedance-2-0-mini", "dreamina-seedance-2-0-mini-260615", "dreamina-seedance-2-0-mini-260615-max"},
	{"dreamina-seedance-2-5-hc", "dreamina-seedance-2-5", "dreamina-seedance-2-5-260628", "dreamina-seedance-2-5-260628-max"},
	{"doubao-seedance-2-0", "doubao-seedance-2-0-260128", "doubao-seedance-2-0-260128-max"},
	{"doubao-seedance-2-0-fast", "doubao-seedance-2-0-fast-260128", "doubao-seedance-2-0-fast-260128-max"},
	{"doubao-seedance-2-0-mini", "doubao-seedance-2-0-mini-260615", "doubao-seedance-2-0-mini-260615-max"},
	{"doubao-seedance-2-5", "doubao-seedance-2-5-260628", "doubao-seedance-2-5-260628-max"},
}

func seedanceAssetFamilyFromWorkflow(workflow string) string {
	switch strings.ToLower(strings.TrimSpace(workflow)) {
	case "dreamina", "hc", "df":
		return seedanceAssetFamilyDreamina
	case "doubao":
		return seedanceAssetFamilyDoubao
	default:
		return ""
	}
}

func seedanceAssetFamilyFromPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/v2/db-sd-max/assets"):
		return seedanceAssetFamilyDoubao
	case strings.HasPrefix(path, "/v2/sd-max/assets"):
		return seedanceAssetFamilyDreamina
	case strings.HasPrefix(path, "/v1/sd-5/assets"):
		return seedanceAssetFamilyDreamina
	case strings.HasPrefix(path, "/v1/sd/assets"):
		return seedanceAssetFamilyDreamina
	default:
		return ""
	}
}

func seedanceAssetFamilyLabel(family string) string {
	switch family {
	case seedanceAssetFamilyDoubao:
		return "豆包"
	case seedanceAssetFamilyDreamina:
		return "Dreamina"
	default:
		return family
	}
}

func seedanceAssetModelCandidates(modelName string) []string {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil
	}
	seen := map[string]struct{}{strings.ToLower(modelName): {}}
	candidates := []string{modelName}
	add := func(name string) {
		name = strings.TrimSpace(name)
		key := strings.ToLower(name)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		candidates = append(candidates, name)
	}
	for _, group := range seedanceAssetAliasGroups {
		matched := false
		for _, name := range group {
			if strings.EqualFold(name, modelName) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		for _, name := range group {
			add(name)
		}
	}
	return candidates
}

func seedanceModelFamily(modelName string) string {
	for _, candidate := range seedanceAssetModelCandidates(modelName) {
		normalized := strings.ToLower(candidate)
		switch {
		case strings.Contains(normalized, "doubao") && strings.Contains(normalized, "seedance"):
			return seedanceAssetFamilyDoubao
		case strings.Contains(normalized, "dreamina") && strings.Contains(normalized, "seedance"):
			return seedanceAssetFamilyDreamina
		}
	}
	return ""
}

func seedanceAssetRoutingGroups(c *gin.Context) []string {
	if c == nil {
		return []string{"default"}
	}
	using := strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	userGroup := strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyUserGroup))
	if using == "auto" {
		if groups := service.GetRequestAutoGroups(c, userGroup); len(groups) > 0 {
			return groups
		}
	}
	if using != "" && using != "auto" {
		return []string{using}
	}
	if userGroup != "" {
		return []string{userGroup}
	}
	return []string{"default"}
}

func seedanceAssetModelFromBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Model      string `json:"Model"`
		LowerModel string `json:"model"`
	}
	if common.Unmarshal(body, &payload) != nil {
		return ""
	}
	if modelName := strings.TrimSpace(payload.Model); modelName != "" {
		return modelName
	}
	return strings.TrimSpace(payload.LowerModel)
}

func channelSeedanceMapping(channel *model.Channel) map[string]string {
	if channel == nil {
		return nil
	}
	raw := strings.TrimSpace(channel.GetModelMapping())
	if raw == "" || raw == "{}" {
		return nil
	}
	mapping := map[string]string{}
	if common.UnmarshalJsonStr(raw, &mapping) != nil {
		return nil
	}
	return mapping
}

func mappingTarget(mapping map[string]string, name string) string {
	if len(mapping) == 0 || strings.TrimSpace(name) == "" {
		return ""
	}
	if target, ok := mapping[name]; ok {
		return strings.TrimSpace(target)
	}
	for key, target := range mapping {
		if strings.EqualFold(key, name) {
			return strings.TrimSpace(target)
		}
	}
	return ""
}

func channelInGroup(channel *model.Channel, group string) bool {
	for _, candidate := range channel.GetGroups() {
		if candidate == group {
			return true
		}
	}
	return false
}

func channelServesSeedanceFamily(channel *model.Channel, family string) bool {
	if channel == nil || family == "" || channel.Type != constant.ChannelTypeServiceInferenceVideo {
		return false
	}
	mapping := channelSeedanceMapping(channel)
	for _, name := range channel.GetModels() {
		if seedanceModelFamily(name) == family || seedanceModelFamily(mappingTarget(mapping, name)) == family {
			return true
		}
	}
	return false
}

func channelMatchesSeedanceModel(channel *model.Channel, modelName string) bool {
	if channel == nil || strings.TrimSpace(modelName) == "" {
		return false
	}
	candidates := seedanceAssetModelCandidates(modelName)
	mapping := channelSeedanceMapping(channel)
	for _, name := range channel.GetModels() {
		target := mappingTarget(mapping, name)
		for _, candidate := range candidates {
			if strings.EqualFold(name, candidate) || strings.EqualFold(target, candidate) {
				return true
			}
		}
	}
	return false
}

func mappedSeedanceAssetModel(channel *model.Channel, modelName string) string {
	modelName = strings.TrimSpace(modelName)
	if channel == nil || modelName == "" {
		return modelName
	}
	mapping := channelSeedanceMapping(channel)
	current := modelName
	seen := map[string]struct{}{}
	for range 8 {
		key := strings.ToLower(current)
		if _, ok := seen[key]; ok {
			return modelName
		}
		seen[key] = struct{}{}
		target := mappingTarget(mapping, current)
		if target == "" || strings.EqualFold(target, current) {
			return current
		}
		current = target
	}
	return modelName
}

func bestSeedanceAssetChannel(channels []*model.Channel) *model.Channel {
	if len(channels) == 0 {
		return nil
	}
	sorted := append([]*model.Channel(nil), channels...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].GetPriority() != sorted[j].GetPriority() {
			return sorted[i].GetPriority() > sorted[j].GetPriority()
		}
		if sorted[i].GetWeight() != sorted[j].GetWeight() {
			return sorted[i].GetWeight() > sorted[j].GetWeight()
		}
		return sorted[i].Id < sorted[j].Id
	})
	return sorted[0]
}

// selectSeedanceAssetChannel picks the type-60 channel whose models match the
// asset family. A higher-priority channel of the other family is ignored,
// because asset:// handles are scoped to the upstream organization.
func selectSeedanceAssetChannel(groups []string, family, modelName string) (*model.Channel, error) {
	family = strings.TrimSpace(family)
	modelName = strings.TrimSpace(modelName)
	if family == "" {
		return nil, fmt.Errorf("缺少素材类型")
	}
	if modelName != "" {
		if modelFamily := seedanceModelFamily(modelName); modelFamily != "" && modelFamily != family {
			return nil, fmt.Errorf("模型 %q 属于%s素材，不能用于%s预上传", modelName, seedanceAssetFamilyLabel(modelFamily), seedanceAssetFamilyLabel(family))
		}
	}
	if len(groups) == 0 {
		groups = []string{"default"}
	}
	if model.DB == nil {
		return nil, fmt.Errorf("没有可用的%s素材渠道", seedanceAssetFamilyLabel(family))
	}
	channels, err := model.GetEnabledChannelsByType(constant.ChannelTypeServiceInferenceVideo)
	if err != nil {
		return nil, fmt.Errorf("查询 ServiceInference 渠道失败: %w", err)
	}
	for _, group := range groups {
		var matches []*model.Channel
		for _, channel := range channels {
			if !channelInGroup(channel, group) || !channelServesSeedanceFamily(channel, family) {
				continue
			}
			if modelName != "" && !channelMatchesSeedanceModel(channel, modelName) {
				continue
			}
			matches = append(matches, channel)
		}
		if channel := bestSeedanceAssetChannel(matches); channel != nil {
			return channel, nil
		}
	}
	if modelName != "" {
		return nil, fmt.Errorf("没有可用于模型 %q 的%s素材渠道", modelName, seedanceAssetFamilyLabel(family))
	}
	return nil, fmt.Errorf("没有可用的%s素材渠道", seedanceAssetFamilyLabel(family))
}
