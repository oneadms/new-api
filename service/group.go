package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if after, ok := strings.CutPrefix(specialGroup, "-:"); ok {
					// 移除分组
					groupToRemove := after
					delete(groupsCopy, groupToRemove)
				} else if after, ok := strings.CutPrefix(specialGroup, "+:"); ok {
					// 添加分组
					groupToAdd := after
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 如果userGroup不在UserUsableGroups中，返回UserUsableGroups + userGroup。
		// 描述是英文源文本，由 web 控制台翻译；管理员配置的描述原样显示。
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "User group"
		}
	}
	return groupsCopy
}

// ResolveUserUsableGroups reuses a request-scoped result so relay authentication,
// playground overrides, model listing, and automatic group selection enforce the
// same subscription restrictions without repeating database work.
func ResolveUserUsableGroups(c *gin.Context, userGroup string) (map[string]string, error) {
	if groups, ok := common.GetContextKeyType[map[string]string](c, constant.ContextKeyUserUsableGroups); ok {
		return groups, nil
	}
	userId := c.GetInt("id")
	groups := GetUserUsableGroups(userGroup)
	// Auto is a routing mode; every selected real group is checked separately.
	groups["auto"] = "Auto"
	if userId <= 0 {
		common.SetContextKey(c, constant.ContextKeyUserUsableGroups, groups)
		return groups, nil
	}
	access, err := model.GetActiveSubscriptionGroupAccess(userId)
	if err != nil {
		return nil, err
	}
	for group := range access.RestrictedGroups {
		delete(groups, group)
	}
	common.SetContextKey(c, constant.ContextKeySubscriptionDisabledGroups, access.SubscriptionDisabledGroups)
	groupPassAccess, err := model.GetActiveGroupPassAccess(userId)
	if err != nil {
		return nil, err
	}
	for group := range groupPassAccess {
		groups[group] = "Temporary speed-pass access"
		common.SetContextKey(c, constant.ContextKeyGroupPassGroup, group)
	}
	common.SetContextKey(c, constant.ContextKeyUserHasActiveSubscription, access.HasActiveSubscription)
	common.SetContextKey(c, constant.ContextKeyUserUsableGroups, groups)
	return groups, nil
}

// ApplyActiveGroupPass returns the temporary group that overrides the
// configured request group while a speed pass is active.
func ApplyActiveGroupPass(c *gin.Context, configuredGroup string) string {
	activeGroup := common.GetContextKeyString(c, constant.ContextKeyGroupPassGroup)
	if activeGroup != "" {
		return activeGroup
	}
	return configuredGroup
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

func IsUserSelectableGroup(userGroup, groupName string) bool {
	if groupName == "" || groupName == "auto" {
		return false
	}
	return GroupInUserUsableGroups(userGroup, groupName) && ratio_setting.ContainsGroupRatio(groupName)
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	return GetUserAutoGroupFromUsableGroups(GetUserUsableGroups(userGroup))
}

func GetUserAutoGroupFromUsableGroups(groups map[string]string) []string {
	autoGroups := make([]string, 0)
	seen := make(map[string]struct{})
	for _, group := range setting.GetAutoGroups() {
		if _, ok := groups[group]; !ok || group == "" || group == "auto" || !ratio_setting.ContainsGroupRatio(group) {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		autoGroups = append(autoGroups, group)
	}
	return autoGroups
}

func ResolveUserAutoGroup(c *gin.Context, userGroup string) ([]string, error) {
	groups, err := ResolveUserUsableGroups(c, userGroup)
	if err != nil {
		return nil, err
	}
	requested := GetRequestAutoGroups(c, userGroup)
	filtered := make([]string, 0, len(requested))
	for _, group := range requested {
		if _, ok := groups[group]; ok {
			filtered = append(filtered, group)
		}
	}
	return filtered, nil
}

// FilterUserTokenAutoGroups applies current permissions before the current
// per-token limit. It intentionally does not fall back to the global Auto list.
func FilterUserTokenAutoGroups(userGroup string, groups []string) []string {
	maxCount := setting.GetMaxTokenAutoGroups()
	filtered := make([]string, 0, min(len(groups), maxCount))
	seen := make(map[string]struct{})
	for _, group := range groups {
		if !IsUserSelectableGroup(userGroup, group) {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		filtered = append(filtered, group)
		if len(filtered) == maxCount {
			break
		}
	}
	return filtered
}

// GetRequestAutoGroups resolves the ordered Auto groups for the current token.
// The absence of the context value means that the token inherits the complete
// global Auto list; a present (even empty) value is an explicit token snapshot.
func GetRequestAutoGroups(c *gin.Context, userGroup string) []string {
	var groups []string
	value, ok := common.GetContextKey(c, constant.ContextKeyTokenAutoGroups)
	if !ok {
		groups = GetUserAutoGroup(userGroup)
	} else if configured, valid := value.([]string); valid {
		groups = FilterUserTokenAutoGroups(userGroup, configured)
	}
	if usable, resolved := common.GetContextKeyType[map[string]string](c, constant.ContextKeyUserUsableGroups); resolved {
		filtered := make([]string, 0, len(groups))
		for _, group := range groups {
			if _, allowed := usable[group]; allowed {
				filtered = append(filtered, group)
			}
		}
		return filtered
	}
	return groups

}

// GetGroupsEnabledModels 按 groups 顺序获取各分组启用的模型并去重
func GetGroupsEnabledModels(groups []string) []string {
	seen := make(map[string]struct{})
	models := make([]string, 0)
	for _, group := range groups {
		for _, modelName := range model.GetGroupEnabledModels(group) {
			if _, ok := seen[modelName]; !ok {
				seen[modelName] = struct{}{}
				models = append(models, modelName)
			}
		}
	}
	return models
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
