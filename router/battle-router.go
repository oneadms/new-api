package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetBattleRouter(router *gin.Engine) {
	battleRoute := router.Group("/api/battle")
	battleRoute.Use(middleware.RouteTag("api"))
	battleRoute.Use(middleware.GlobalAPIRateLimit())
	{
		battleRoute.GET("/status", middleware.UserAuth(), controller.GetBattleStatus)
		battleRoute.GET("/ws", controller.BattleWebSocket)
	}
}
