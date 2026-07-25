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

// ResolveUserUsableGroups reuses a request-scoped result so relay authentication,
// playground overrides, model listing, and automatic group selection enforce the
// same subscription restrictions without repeating database work.
func ResolveUserUsableGroups(c *gin.Context, userGroup string) (map[string]string, error) {
	if groups, ok := common.GetContextKeyType[map[string]string](c, constant.ContextKeyUserUsableGroups); ok {
		return groups, nil
	}
	userId := c.GetInt("id")
	groups := GetUserUsableGroups(userGroup)
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

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	return GetUserAutoGroupFromUsableGroups(GetUserUsableGroups(userGroup))
}

func GetUserAutoGroupFromUsableGroups(groups map[string]string) []string {
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

func ResolveUserAutoGroup(c *gin.Context, userGroup string) ([]string, error) {
	groups, err := ResolveUserUsableGroups(c, userGroup)
	if err != nil {
		return nil, err
	}
	return GetUserAutoGroupFromUsableGroups(groups), nil
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
