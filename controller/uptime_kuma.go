package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/console_setting"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

const (
	requestTimeout      = 30 * time.Second
	httpTimeout         = 10 * time.Second
	uptime24KeySuffix   = "_24"
	uptime30KeySuffix   = "_720"
	apiStatusPath       = "/api/status-page/"
	apiHeartbeatPath    = "/api/status-page/heartbeat/"
	maxHeartbeatSamples = 720
)

type Heartbeat struct {
	Status int    `json:"status"`
	Time   string `json:"time,omitempty"`
}

type Monitor struct {
	Name       string      `json:"name"`
	Uptime     float64     `json:"uptime"`
	Uptime24   *float64    `json:"uptime24,omitempty"`
	Uptime30   *float64    `json:"uptime30,omitempty"`
	Status     int         `json:"status"`
	Group      string      `json:"group,omitempty"`
	Heartbeats []Heartbeat `json:"heartbeats,omitempty"`
}

type UptimeGroupResult struct {
	CategoryName string    `json:"categoryName"`
	Monitors     []Monitor `json:"monitors"`
}

func getAndDecode(ctx context.Context, client *http.Client, url string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("non-200 status")
	}

	return common.DecodeJson(resp.Body, dest)
}

func trimHeartbeats(heartbeats []Heartbeat) []Heartbeat {
	if len(heartbeats) <= maxHeartbeatSamples {
		return heartbeats
	}
	return heartbeats[:maxHeartbeatSamples]
}

func float64Ptr(value float64) *float64 {
	return &value
}

func fetchGroupData(ctx context.Context, client *http.Client, groupConfig map[string]interface{}) UptimeGroupResult {
	url, _ := groupConfig["url"].(string)
	slug, _ := groupConfig["slug"].(string)
	categoryName, _ := groupConfig["categoryName"].(string)

	result := UptimeGroupResult{
		CategoryName: categoryName,
		Monitors:     []Monitor{},
	}

	if url == "" || slug == "" {
		return result
	}

	baseURL := strings.TrimSuffix(url, "/")

	var statusData struct {
		PublicGroupList []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			MonitorList []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"monitorList"`
		} `json:"publicGroupList"`
	}

	var heartbeatData struct {
		HeartbeatList map[string][]Heartbeat `json:"heartbeatList"`
		UptimeList    map[string]float64     `json:"uptimeList"`
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return getAndDecode(gCtx, client, baseURL+apiStatusPath+slug, &statusData)
	})
	g.Go(func() error {
		return getAndDecode(gCtx, client, baseURL+apiHeartbeatPath+slug, &heartbeatData)
	})

	if g.Wait() != nil {
		return result
	}

	for _, pg := range statusData.PublicGroupList {
		if len(pg.MonitorList) == 0 {
			continue
		}

		for _, m := range pg.MonitorList {
			monitor := Monitor{
				Name:   m.Name,
				Group:  pg.Name,
				Status: -1,
			}

			monitorID := strconv.Itoa(m.ID)

			if uptime, exists := heartbeatData.UptimeList[monitorID+uptime24KeySuffix]; exists {
				monitor.Uptime = uptime
				monitor.Uptime24 = float64Ptr(uptime)
			}

			if uptime, exists := heartbeatData.UptimeList[monitorID+uptime30KeySuffix]; exists {
				monitor.Uptime30 = float64Ptr(uptime)
			}

			if heartbeats, exists := heartbeatData.HeartbeatList[monitorID]; exists && len(heartbeats) > 0 {
				monitor.Heartbeats = trimHeartbeats(heartbeats)
				monitor.Status = heartbeats[0].Status
			}

			result.Monitors = append(result.Monitors, monitor)
		}
	}

	return result
}

func GetUptimeKumaStatus(c *gin.Context) {
	groups := console_setting.GetUptimeKumaGroups()
	if len(groups) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": []UptimeGroupResult{}})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), requestTimeout)
	defer cancel()

	client := &http.Client{Timeout: httpTimeout}
	results := make([]UptimeGroupResult, len(groups))

	g, gCtx := errgroup.WithContext(ctx)
	for i, group := range groups {
		i, group := i, group
		g.Go(func() error {
			results[i] = fetchGroupData(gCtx, client, group)
			return nil
		})
	}

	g.Wait()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": results})
}
