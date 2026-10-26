package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func parseStatusQuery(c *gin.Context) (service.StatusPageQuery, error) {
	query := service.StatusPageQuery{Hours: 24, Page: 1, PageSize: 20, Sort: "name",
		Model: c.Query("model"), Group: c.Query("group"), State: c.Query("state")}
	for name, target := range map[string]*int{"hours": &query.Hours, "page": &query.Page, "page_size": &query.PageSize} {
		if value, present := c.GetQuery(name); present {
			number, err := strconv.Atoi(value)
			if err != nil {
				return query, &service.QualityValidationError{Message: "Invalid status page filters"}
			}
			*target = number
		}
	}
	if value, present := c.GetQuery("sort"); present {
		query.Sort = value
	}
	return query, service.ValidateStatusPageQuery(query)
}

func statusPageError(c *gin.Context, err error) {
	code, message := http.StatusServiceUnavailable, "Status page data is unavailable"
	var validation *service.QualityValidationError
	if errors.As(err, &validation) {
		code, message = http.StatusBadRequest, validation.Message
	}
	if errors.Is(err, service.ErrStatusPageNotFound) || errors.Is(err, service.ErrQualityArtifactUnavailable) {
		code, message = http.StatusNotFound, "Published status resource not found"
	}
	c.JSON(code, gin.H{"success": false, "message": message})
}

func GetNativeStatusPage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	query, err := parseStatusQuery(c)
	if err != nil {
		statusPageError(c, err)
		return
	}
	data, err := service.GetStatusPage(c.Request.Context(), query)
	if err != nil {
		statusPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func GetNativeStatusGroup(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	query, err := parseStatusQuery(c)
	if err != nil {
		statusPageError(c, err)
		return
	}
	data, err := service.GetStatusGroup(c.Request.Context(), c.Param("public_id"), query)
	if err != nil {
		statusPageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func GetNativeStatusThumbnail(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	data, err := service.ReadPublicStatusThumbnail(c.Param("public_id"))
	if err != nil {
		statusPageError(c, err)
		return
	}
	c.Data(http.StatusOK, "image/png", data)
}
