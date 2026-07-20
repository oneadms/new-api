package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/pkg/ssetrace"

	"github.com/gin-gonic/gin"
)

func GetSSETrace(c *gin.Context) {
	requestID := strings.TrimSpace(c.Param("request_id"))
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "request id is required",
		})
		return
	}

	trace, err := ssetrace.Read(requestID)
	if err != nil {
		if errors.Is(err, ssetrace.ErrTraceNotFound) || errors.Is(err, ssetrace.ErrTraceExpired) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "SSE trace not found or expired",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    trace,
	})
}

func GetSSETraceStats(c *gin.Context) {
	stats, err := ssetrace.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

func CleanupExpiredSSETraces(c *gin.Context) {
	if err := ssetrace.CleanupExpired(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
