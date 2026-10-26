package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func qualityControllerFixture(t *testing.T) *gin.Engine {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB := model.DB
	mainType, logType := common.MainDatabaseType(), common.LogDatabaseType()
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = oldDB
		common.SetDatabaseTypes(mainType, logType)
		require.NoError(t, sqlDB.Close())
	})
	t.Setenv("QUALITY_ARTIFACT_DIR", t.TempDir())
	t.Setenv("QUALITY_RENDERER_URL", "")
	t.Setenv("QUALITY_RENDERER_TOKEN", "")
	require.NoError(t, db.AutoMigrate(&model.QualitySettings{}, &model.QualityProbeRun{}, &model.QualityProbeBatch{}, &model.QualityPromptVersion{}))
	router := gin.New()
	router.GET("/settings", QualityGetSettings)
	router.PUT("/settings", QualityUpdateSettings)
	router.GET("/batches", QualityListBatches)
	router.GET("/runs/:id", QualityGetRun)
	router.GET("/runs/:id/output", QualityGetOutput)
	router.GET("/runs/:id/thumbnail", QualityGetThumbnail)
	router.POST("/batches", QualityCreateBatch)
	router.POST("/prompts", QualityCreatePrompt)
	return router
}

func TestQualitySettingsHTTPVersionConflictUses409AndNoStore(t *testing.T) {
	router := qualityControllerFixture(t)
	settings, err := service.GetQualitySettings()
	require.NoError(t, err)
	data, err := common.Marshal(settings)
	require.NoError(t, err)
	for _, expected := range []int{http.StatusOK, http.StatusConflict} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewReader(data))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		assert.Equal(t, expected, response.Code)
		assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		var envelope struct {
			Success bool `json:"success"`
			Data    any  `json:"data"`
		}
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &envelope))
		assert.Equal(t, expected == http.StatusOK, envelope.Success)
	}
}

func TestQualityOutputDownloadsAsPlainTextAndMetadataNeverContainsSourceOrPaths(t *testing.T) {
	router := qualityControllerFixture(t)
	source := []byte("<html><script>window.privateSource=true</script></html>")
	digest := sha256.Sum256(source)
	key := common.GetUUID() + "-" + hex.EncodeToString(digest[:]) + ".html"
	directory := os.Getenv("QUALITY_ARTIFACT_DIR")
	require.NoError(t, os.WriteFile(filepath.Join(directory, key), source, 0600))
	run := model.QualityProbeRun{PublicID: "public-run", BatchID: 1, ChannelID: 1, Status: "completed", HasHTML: true, HTMLKey: key,
		PromptSnapshot: "private prompt content", GroupsJSON: `["private-group"]`}
	require.NoError(t, model.DB.Create(&run).Error)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/runs/1/output?kind=html", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, source, response.Body.Bytes())
	assert.Equal(t, "text/plain; charset=utf-8", response.Header().Get("Content-Type"))
	assert.Contains(t, response.Header().Get("Content-Disposition"), "attachment;")
	assert.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
	assert.Contains(t, response.Header().Get("Content-Security-Policy"), "sandbox")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/runs/1", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	for _, private := range []string{key, directory, "privateSource", "private prompt", "private-group", "html_key", "prompt_snapshot"} {
		assert.NotContains(t, response.Body.String(), private)
	}
}

func TestQualityHTTPRejectsMalformedPayloadPaginationAndInvalidArtifactKinds(t *testing.T) {
	router := qualityControllerFixture(t)
	cases := []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPut, "/settings", `{`, http.StatusBadRequest},
		{http.MethodPut, "/settings", strings.Repeat("x", (128<<10)+1), http.StatusBadRequest},
		{http.MethodGet, "/batches?page=-1", "", http.StatusBadRequest},
		{http.MethodGet, "/batches?page_size=1000", "", http.StatusBadRequest},
		{http.MethodGet, "/runs/not-a-number", "", http.StatusBadRequest},
		{http.MethodGet, "/runs/999", "", http.StatusNotFound},
		{http.MethodGet, "/runs/1/output?kind=../../private", "", http.StatusBadRequest},
		{http.MethodPost, "/batches", `{}`, http.StatusBadRequest},
	}
	for _, tt := range cases {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			assert.Equal(t, tt.status, response.Code)
			assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
			assert.Contains(t, response.Body.String(), `"success":false`)
		})
	}
}
