package controller

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	battlesvc "github.com/QuantumNous/new-api/service/battle"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var battleUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		parsed, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return strings.EqualFold(parsed.Host, r.Host)
	},
}

func GetBattleStatus(c *gin.Context) {
	userId := c.GetInt("id")
	quota, err := model.GetUserQuota(userId, true)
	if err != nil {
		common.SysLog("failed to get battle user quota: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	usage, err := model.GetBattleDailyQuotaUsage(userId)
	if err != nil {
		common.SysLog("failed to get battle daily quota usage: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	setting := operation_setting.GetBattleSetting()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"enabled":         setting.Enabled,
			"hide_room_input": setting.HideRoomInput,
			"quota":           quota,
			"daily_lost":      usage.Lost,
			"daily_won":       usage.Won,
			"max_round_loss":  setting.MaxRoundLossQuota,
			"max_round_gain":  setting.MaxRoundGainQuota,
			"max_daily_loss":  setting.MaxDailyLossQuota,
			"max_daily_gain":  setting.MaxDailyGainQuota,
			"cap_quota":       setting.CapQuota,
			"max_players":     setting.MaxPlayersPerRoom,
			"map_width":       setting.MapWidth,
			"map_height":      setting.MapHeight,
			"player_speed":    setting.PlayerSpeed,
		},
	})
}

func BattleWebSocket(c *gin.Context) {
	if !operation_setting.IsBattleEnabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "battle disabled",
		})
		return
	}

	userId, username, ok := getBattleSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "not logged in",
		})
		return
	}

	quota, err := model.GetUserQuota(userId, true)
	if err != nil {
		common.SysLog("failed to get battle user quota before websocket: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if quota <= 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"success": false,
			"message": "Balance must be positive to join",
		})
		return
	}

	ws, err := battleUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		common.SysLog("failed to upgrade battle websocket: " + err.Error())
		return
	}

	battlesvc.DefaultManager.Join(ws, userId, username, c.Query("room"))
}

func getBattleSessionUser(c *gin.Context) (int, string, bool) {
	session := sessions.Default(c)
	idValue := session.Get("id")
	usernameValue := session.Get("username")
	statusValue := session.Get("status")
	if idValue == nil || usernameValue == nil || statusValue == nil {
		return 0, "", false
	}

	userId, ok := idValue.(int)
	if !ok || userId <= 0 {
		return 0, "", false
	}
	username, ok := usernameValue.(string)
	if !ok || username == "" {
		return 0, "", false
	}
	status, ok := statusValue.(int)
	if !ok || status == common.UserStatusDisabled {
		return 0, "", false
	}
	return userId, username, true
}
