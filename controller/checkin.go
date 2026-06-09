package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type CheckinRequest struct {
	StakeQuota *int `json:"stake_quota,omitempty"`
}

// GetCheckinStatus 获取用户签到状态和历史记录
func GetCheckinStatus(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	luckySetting := operation_setting.GetLuckyCheckinSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "签到功能未启用")
		return
	}
	userId := c.GetInt("id")
	// 获取月份参数，默认为当前月份
	month := c.DefaultQuery("month", time.Now().Format("2006-01"))

	stats, err := model.GetUserCheckinStats(userId, month)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":   setting.Enabled,
			"min_quota": setting.MinQuota,
			"max_quota": setting.MaxQuota,
			"lucky": gin.H{
				"enabled":         luckySetting.Enabled,
				"min_stake_quota": luckySetting.MinStakeQuota,
				"max_stake_quota": luckySetting.MaxStakeQuota,
				"min_failure_bps": luckySetting.MinFailureBps,
				"max_failure_bps": luckySetting.MaxFailureBps,
			},
			"stats": stats,
		},
	})
}

// DoCheckin 执行用户签到
func DoCheckin(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "签到功能未启用")
		return
	}

	userId := c.GetInt("id")
	var request CheckinRequest
	if c.Request.ContentLength != 0 {
		if err := common.DecodeJson(c.Request.Body, &request); err != nil {
			common.ApiErrorMsg(c, "无效的签到参数")
			return
		}
	}

	checkin, err := model.UserCheckin(userId, request.StakeQuota)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	action := "用户签到，获得额度"
	if request.StakeQuota != nil {
		action = "用户运气签到，额度变化"
	}
	model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("%s %s", action, logger.LogQuota(checkin.QuotaAwarded)))
	won := checkin.QuotaAwarded >= 0
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "签到成功",
		"data": gin.H{
			"quota_awarded": checkin.QuotaAwarded,
			"checkin_date":  checkin.CheckinDate,
			"lucky":         request.StakeQuota != nil,
			"won":           won,
		},
	})
}
