package model_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenAuthRejectsExistingTokensUsingSubscriptionRestrictedGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))

	testCases := []struct {
		name       string
		userId     int
		userGroup  string
		tokenGroup string
	}{
		{name: "explicit token group", userId: 2601, userGroup: "default", tokenGroup: "vip"},
		{name: "empty token group inherits user group", userId: 2602, userGroup: "vip", tokenGroup: ""},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Cleanup(func() {
				require.NoError(t, model.DB.Where("user_id = ?", testCase.userId).Delete(&model.UserSubscription{}).Error)
				require.NoError(t, model.DB.Where("user_id = ?", testCase.userId).Delete(&model.Token{}).Error)
				require.NoError(t, model.DB.Unscoped().Delete(&model.User{}, testCase.userId).Error)
				require.NoError(t, model.DB.Delete(&model.SubscriptionPlan{}, testCase.userId).Error)
				require.NoError(t, model.InvalidateUserCache(testCase.userId))
				model.InvalidateSubscriptionPlanCache(testCase.userId)
			})

			user := model.User{
				Id: testCase.userId, Username: fmt.Sprintf("restricted-token-user-%d", testCase.userId),
				Password: "password", AffCode: fmt.Sprintf("restricted-aff-%d", testCase.userId), Group: testCase.userGroup,
				Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AuthVersion: 1,
			}
			require.NoError(t, model.DB.Create(&user).Error)
			plan := model.SubscriptionPlan{
				Id: testCase.userId, Title: "existing token restriction plan", DurationUnit: model.SubscriptionDurationMonth,
				DurationValue: 1, RestrictedGroups: []string{"vip"},
			}
			require.NoError(t, model.DB.Create(&plan).Error)
			model.InvalidateSubscriptionPlanCache(plan.Id)
			now := common.GetTimestamp()
			require.NoError(t, model.DB.Create(&model.UserSubscription{
				UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600,
			}).Error)
			token := model.Token{
				UserId: user.Id, Key: fmt.Sprintf("restricttoken%d", testCase.userId), Name: "existing restricted token",
				Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, Group: testCase.tokenGroup,
			}
			require.NoError(t, model.DB.Create(&token).Error)

			router := gin.New()
			router.GET("/protected", middleware.TokenAuth(), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", "Bearer "+token.Key)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			assert.Equal(t, http.StatusForbidden, response.Code)
			assert.Contains(t, response.Body.String(), "无权访问 vip 分组")
		})
	}
}
