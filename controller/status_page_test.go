package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func statusControllerFixture(t *testing.T) (*gin.Engine, string, []model.QualityProbeRun) {
	t.Helper()
	router := qualityControllerFixture(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.ChannelQualityProfile{}, &model.QualityProbeRunGroup{}, &model.Ability{}, &model.PerfMetric{}))
	settings, err := service.GetQualitySettings()
	require.NoError(t, err)
	settings.StatusPageEnabled = true
	settings.PublicGroups = []model.QualityPublicGroup{{Name: "private-group", Alias: "Public group"}}
	_, err = service.UpdateQualitySettings(*settings)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 1, Group: "private-group", Models: "text-model", Name: "private-name", Key: "private-key"}).Error)
	require.NoError(t, model.DB.Create(&model.ChannelQualityProfile{ChannelID: 1, PublicAlias: "Public channel", PublishSample: true}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{ChannelId: 1, Group: "private-group", Model: "text-model"}).Error)
	runs := []model.QualityProbeRun{
		{PublicID: common.GetUUID(), BatchID: 1, ChannelID: 1, Model: "text-model", Status: "failed", CompletedAt: time.Now().Unix() - 48*3600, PromptSnapshot: "private-prompt"},
		{PublicID: common.GetUUID(), BatchID: 2, ChannelID: 1, Model: "text-model", Status: "completed", CompletedAt: time.Now().Unix(), ChannelName: "private-name", HTMLKey: "private-html-key"},
	}
	for i := range runs {
		require.NoError(t, model.DB.Create(&runs[i]).Error)
		require.NoError(t, model.DB.Create(&model.QualityProbeRunGroup{RunID: runs[i].ID, GroupName: "private-group"}).Error)
	}
	router.GET("/api/status-page", GetNativeStatusPage)
	router.GET("/api/status-page/groups/:public_id", GetNativeStatusGroup)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/status-page", nil))
	require.Equal(t, http.StatusOK, response.Code)
	var envelope struct {
		Data service.StatusPageData `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Groups, 1)
	return router, "/api/status-page/groups/" + envelope.Data.Groups[0].PublicID, runs
}

func TestStatusGroupHTTPReturnsHistoricalPaginationAndEmptyPage(t *testing.T) {
	router, path, runs := statusControllerFixture(t)
	for _, tt := range []struct {
		query          string
		page, pageSize int
		ids            []string
	}{
		{"", 1, 20, []string{runs[1].PublicID, runs[0].PublicID}},
		{"?hours=1&page=2&page_size=1", 2, 1, []string{runs[0].PublicID}},
		{"?page=3&page_size=1", 3, 1, []string{}},
	} {
		t.Run(tt.query, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path+tt.query, nil))
			require.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
			var envelope struct {
				Success bool `json:"success"`
				Data    struct {
					Group    service.StatusGroup    `json:"group"`
					Samples  []service.StatusSample `json:"samples"`
					Total    *int64                 `json:"total"`
					Page     *int                   `json:"page"`
					PageSize *int                   `json:"page_size"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &envelope))
			assert.True(t, envelope.Success)
			ids := make([]string, 0, len(envelope.Data.Samples))
			for _, sample := range envelope.Data.Samples {
				ids = append(ids, sample.PublicID)
				assert.Nil(t, sample.ThumbnailURL)
			}
			assert.Equal(t, tt.ids, ids)
			expectedTotal := int64(2)
			assert.Equal(t, &expectedTotal, envelope.Data.Total)
			assert.Equal(t, &tt.page, envelope.Data.Page)
			assert.Equal(t, &tt.pageSize, envelope.Data.PageSize)
			assert.NotNil(t, envelope.Data.Samples)
			require.NotNil(t, envelope.Data.Group.LatestSample)
			assert.Equal(t, runs[1].PublicID, envelope.Data.Group.LatestSample.PublicID)
			assert.NotContains(t, response.Body.String(), "private-")
		})
	}
}

func TestStatusGroupHTTPRejectsInvalidPagination(t *testing.T) {
	router, path, _ := statusControllerFixture(t)
	for _, query := range []string{"page=0", "page=-1", "page=100001", "page=no", "page=99999999999999999999999", "page=", "page_size=0", "page_size=-1", "page_size=101", "page_size=no", "page_size="} {
		t.Run(query, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path+"?"+query, nil))
			assert.Equal(t, http.StatusBadRequest, response.Code)
			assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
			assert.Contains(t, response.Body.String(), `"success":false`)
		})
	}
}
