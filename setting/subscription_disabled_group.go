package setting

import (
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var (
	subscriptionDisabledGroups      = []string{}
	subscriptionDisabledGroupsMutex sync.RWMutex
)

func UpdateSubscriptionDisabledGroupsByJSONString(jsonString string) error {
	var groups []string
	if err := common.Unmarshal([]byte(jsonString), &groups); err != nil {
		return err
	}

	uniqueGroups := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group != "" {
			uniqueGroups[group] = struct{}{}
		}
	}

	normalizedGroups := make([]string, 0, len(uniqueGroups))
	for group := range uniqueGroups {
		normalizedGroups = append(normalizedGroups, group)
	}
	sort.Strings(normalizedGroups)

	subscriptionDisabledGroupsMutex.Lock()
	subscriptionDisabledGroups = normalizedGroups
	subscriptionDisabledGroupsMutex.Unlock()
	return nil
}

func SubscriptionDisabledGroups2JSONString() string {
	subscriptionDisabledGroupsMutex.RLock()
	groups := append([]string(nil), subscriptionDisabledGroups...)
	subscriptionDisabledGroupsMutex.RUnlock()

	jsonBytes, err := common.Marshal(groups)
	if err != nil {
		common.SysLog("error marshalling subscription disabled groups: " + err.Error())
		return "[]"
	}
	return string(jsonBytes)
}

func IsSubscriptionDisabledGroup(group string) bool {
	subscriptionDisabledGroupsMutex.RLock()
	defer subscriptionDisabledGroupsMutex.RUnlock()

	for _, disabledGroup := range subscriptionDisabledGroups {
		if disabledGroup == group {
			return true
		}
	}
	return false
}
