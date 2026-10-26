package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func qualityRespond(c *gin.Context, status int, data any) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(status, gin.H{"success": true, "message": "", "data": data})
}

func qualityRespondError(c *gin.Context, err error, data any) {
	status, message := http.StatusInternalServerError, "Quality detection operation failed"
	var invalid *service.QualityValidationError
	switch {
	case errors.As(err, &invalid):
		status, message = http.StatusBadRequest, invalid.Message
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, service.ErrQualityArtifactUnavailable):
		status, message = http.StatusNotFound, "Quality detection resource not found"
	case errors.Is(err, model.ErrQualityIdempotency):
		status, message = http.StatusConflict, "Idempotency-Key was already used for a different request"
	case errors.Is(err, model.ErrQualityConflict):
		status, message = http.StatusConflict, "Configuration changed or another quality detection batch is active"
	case errors.Is(err, model.ErrQualityBudget):
		status, message = http.StatusConflict, "Daily quality detection budget exhausted"
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(status, gin.H{"success": false, "message": message, "data": data})
}

func qualityDecode(c *gin.Context, value any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	data, err := io.ReadAll(c.Request.Body)
	trimmed := bytes.TrimSpace(data)
	if err == nil && (len(trimmed) == 0 || trimmed[0] != '{') {
		err = errors.New("expected a JSON object")
	}
	if err == nil {
		err = common.Unmarshal(data, value)
	}
	if err != nil {
		qualityRespondError(c, &service.QualityValidationError{Message: "Invalid or oversized JSON body"}, nil)
		return false
	}
	return true
}

func qualityID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		qualityRespondError(c, &service.QualityValidationError{Message: "Invalid resource ID"}, nil)
		return 0, false
	}
	return id, true
}

func QualityGetSettings(c *gin.Context) {
	settings, err := service.GetQualitySettings()
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, settings)
}

func QualityUpdateSettings(c *gin.Context) {
	var input model.QualitySettings
	if !qualityDecode(c, &input) {
		return
	}
	settings, err := service.UpdateQualitySettings(input)
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, settings)
}

func QualityListChannels(c *gin.Context) {
	channels, err := service.ListQualityChannels()
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, channels)
}

func QualityUpdateChannel(c *gin.Context) {
	id, ok := qualityID(c)
	if !ok {
		return
	}
	if id > 2147483647 {
		qualityRespondError(c, &service.QualityValidationError{Message: "Invalid channel ID"}, nil)
		return
	}
	var input model.ChannelQualityProfile
	if !qualityDecode(c, &input) {
		return
	}
	profile, err := service.UpdateQualityChannel(int(id), input)
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, profile)
}

func QualityListPrompts(c *gin.Context) {
	prompts, err := service.ListQualityPrompts()
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, prompts)
}

func QualityCreatePrompt(c *gin.Context) {
	var input struct {
		Key     string `json:"key"`
		Name    string `json:"name"`
		Content string `json:"content"`
		Rule    string `json:"rule"`
	}
	if !qualityDecode(c, &input) {
		return
	}
	prompt, err := service.CreateQualityPrompt(input.Key, input.Name, input.Content, input.Rule, c.GetInt("id"))
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusCreated, prompt)
}

func QualityArchivePrompt(c *gin.Context) {
	id, ok := qualityID(c)
	if !ok {
		return
	}
	var input struct {
		Archived *bool `json:"archived"`
	}
	if !qualityDecode(c, &input) {
		return
	}
	if input.Archived == nil {
		qualityRespondError(c, &service.QualityValidationError{Message: "archived is required"}, nil)
		return
	}
	if err := model.ArchiveQualityPrompt(id, *input.Archived); err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, nil)
}

func QualityCreateBatch(c *gin.Context) {
	var input service.QualityBatchRequest
	if !qualityDecode(c, &input) {
		return
	}
	batch, err := service.CreateQualityBatch(input, c.GetHeader("Idempotency-Key"), c.GetInt("id"))
	if err != nil {
		qualityRespondError(c, err, batch)
		return
	}
	qualityRespond(c, http.StatusAccepted, batch)
}

func QualityListBatches(c *gin.Context) {
	page, errPage := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, errSize := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if errPage != nil || errSize != nil || page < 1 || page > 1000000 || size < 1 || size > 100 {
		qualityRespondError(c, &service.QualityValidationError{Message: "Invalid pagination"}, nil)
		return
	}
	items, total, err := model.ListQualityBatches(page, size)
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}

func QualityGetBatch(c *gin.Context) {
	id, ok := qualityID(c)
	if !ok {
		return
	}
	batch, runs, err := model.GetQualityBatch(id)
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, gin.H{"batch": batch, "runs": runs})
}

func QualityCancelBatch(c *gin.Context) {
	id, ok := qualityID(c)
	if !ok {
		return
	}
	if err := model.CancelQualityBatch(id); err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, nil)
}

func QualityGetRun(c *gin.Context) {
	id, ok := qualityID(c)
	if !ok {
		return
	}
	run, err := service.GetQualityRun(id)
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, run)
}

func QualityGetOutput(c *gin.Context) {
	id, ok := qualityID(c)
	if !ok {
		return
	}
	kind := c.DefaultQuery("kind", "raw")
	if kind != "raw" && kind != "html" {
		qualityRespondError(c, &service.QualityValidationError{Message: "Invalid output kind"}, nil)
		return
	}
	run, err := service.GetQualityRun(id)
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	data, err := service.ReadQualityArtifact(run, kind)
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox; default-src 'none'")
	// Even HTML is an attachment with text/plain; it never executes on the
	// administrative origin. No caller-controlled filename is used.
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"quality-%d-%s.txt\"", id, kind))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", data)
}

func QualityGetThumbnail(c *gin.Context) {
	id, ok := qualityID(c)
	if !ok {
		return
	}
	run, err := service.GetQualityRun(id)
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	data, err := service.ReadQualityArtifact(run, "thumbnail")
	if err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Data(http.StatusOK, "image/png", data)
}

func QualityReviewRun(c *gin.Context) {
	id, ok := qualityID(c)
	if !ok {
		return
	}
	var input struct {
		Result string `json:"result"`
		Note   string `json:"note"`
	}
	if !qualityDecode(c, &input) {
		return
	}
	if err := service.ReviewQualityRun(id, c.GetInt("id"), input.Result, input.Note); err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, nil)
}

func QualitySetBaseline(c *gin.Context) {
	id, ok := qualityID(c)
	if !ok {
		return
	}
	if err := service.SetQualityBaseline(id, c.GetInt("id")); err != nil {
		qualityRespondError(c, err, nil)
		return
	}
	qualityRespond(c, http.StatusOK, nil)
}
