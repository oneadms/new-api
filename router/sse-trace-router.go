package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetSSETraceRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	apiRouter.GET("/log/sse_trace/:request_id", middleware.AdminAuth(), controller.GetSSETrace)

	performanceRoute := apiRouter.Group("/performance")
	performanceRoute.Use(middleware.RootAuth())
	performanceRoute.GET("/sse_traces", controller.GetSSETraceStats)
	performanceRoute.DELETE("/sse_traces/expired", controller.CleanupExpiredSSETraces)
}
