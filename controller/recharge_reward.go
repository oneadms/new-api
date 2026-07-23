package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetRechargeRewardSelf(c *gin.Context) {
	rewards, err := model.GetUserRechargeRewards(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rewards)
}

func ActivateGroupPass(c *gin.Context) {
	passId, err := strconv.Atoi(c.Param("id"))
	if err != nil || passId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "速通卡 ID 无效"})
		return
	}
	pass, err := model.ActivateUserGroupPass(c.GetInt("id"), passId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	common.ApiSuccess(c, pass)
}

func DrawRechargeLottery(c *gin.Context) {
	result, err := model.DrawRechargeLottery(c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	common.ApiSuccess(c, result)
}

func AdminGetRechargeRewardSettings(c *gin.Context) {
	settings, err := model.GetRechargeRewardSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, settings)
}

func AdminSaveRechargeRewardSettings(c *gin.Context) {
	var settings model.RechargeRewardSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "奖励配置参数无效"})
		return
	}
	saved, err := model.SaveRechargeRewardSettings(settings)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	recordManageAudit(c, "recharge_reward.settings.update", map[string]interface{}{
		"version": saved.Version,
	})
	common.ApiSuccess(c, saved)
}

func AdminGrantGroupPass(c *gin.Context) {
	var request model.GroupPassGrantRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "发放参数无效"})
		return
	}
	passes, err := model.GrantUserGroupPasses(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	recordManageAudit(c, "recharge_reward.group_pass.grant", map[string]interface{}{
		"user_id":     request.UserId,
		"template_id": request.TemplateId,
		"quantity":    len(passes),
		"expires_at":  request.ExpiresAt,
	})
	common.ApiSuccess(c, passes)
}
