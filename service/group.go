package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					// 移除分组
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					// 添加分组
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 如果userGroup不在UserUsableGroups中，返回UserUsableGroups + userGroup
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "用户分组"
		}
	}
	return groupsCopy
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserUsableGroups(userGroup)
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

// GetUserAutoGroupWithPriority 根据用户分组和令牌自定义优先级获取自动分组设置
// customPriorityJson: 令牌存储的自定义优先级 JSON 数组，如 `["vip","default"]`
// 如果 customPriorityJson 为空，返回系统默认顺序
// 自定义优先级中的分组会排在前面，系统默认中剩余的分组会追加在后面
func GetUserAutoGroupWithPriority(userGroup string, customPriorityJson string) []string {
	groups := GetUserUsableGroups(userGroup)

	// 如果令牌没有自定义优先级，直接使用系统默认
	if customPriorityJson == "" {
		return GetUserAutoGroup(userGroup)
	}

	// 解析令牌的自定义优先级
	var customPriority []string
	if err := common.Unmarshal([]byte(customPriorityJson), &customPriority); err != nil {
		// 解析失败时回退到系统默认
		return GetUserAutoGroup(userGroup)
	}

	seen := make(map[string]bool)
	result := make([]string, 0)

	// 先按令牌自定义顺序添加用户可用的分组
	for _, g := range customPriority {
		if _, ok := groups[g]; ok {
			if !seen[g] {
				seen[g] = true
				result = append(result, g)
			}
		}
	}

	// 追加系统默认列表中用户可用但未被自定义列表覆盖的分组
	for _, g := range setting.GetAutoGroups() {
		if _, ok := groups[g]; ok {
			if !seen[g] {
				seen[g] = true
				result = append(result, g)
			}
		}
	}

	if len(result) == 0 {
		return GetUserAutoGroup(userGroup)
	}
	return result
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
func GetUserGroupRatio(userGroup, group string) float64 {
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(group)
}
