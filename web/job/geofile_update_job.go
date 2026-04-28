package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// GeofileUpdateJob handles scheduled geofile updates.
type GeofileUpdateJob struct {
	settingService service.SettingService
	serverService  service.ServerService
}

// NewGeofileUpdateJob creates a new GeofileUpdateJob instance.
func NewGeofileUpdateJob() *GeofileUpdateJob {
	return &GeofileUpdateJob{}
}

// Run executes the scheduled geofile update if enabled and time matches the configured frequency.
func (j *GeofileUpdateJob) Run() {
	enabled, err := j.settingService.GetGeofileUpdateEnabled()
	if err != nil || !enabled {
		return
	}

	frequency, err := j.settingService.GetGeofileUpdateFrequency()
	if err != nil {
		return
	}

	if !j.shouldRun(frequency) {
		return
	}

	if err := j.serverService.UpdateGeofile(""); err != nil {
		logger.Warning("scheduled geofile update failed:", err)
		return
	}

	if err := database.BumpSharedGeoVersion(database.GetDB()); err != nil {
		logger.Warning("bump shared geo version failed:", err)
	}
}

// shouldRun checks if the geofile update should run based on the configured frequency.
func (j *GeofileUpdateJob) shouldRun(frequency string) bool {
	now := time.Now()

	switch frequency {
	case "hourly":
		return now.Minute() == 0
	case "every12h":
		return (now.Hour() == 0 || now.Hour() == 12) && now.Minute() == 0
	case "daily":
		hour, err := j.settingService.GetGeofileUpdateHour()
		if err != nil {
			hour = 4
		}
		return now.Hour() == hour && now.Minute() == 0
	case "weekly":
		if now.Weekday() != time.Sunday {
			return false
		}
		hour, err := j.settingService.GetGeofileUpdateHour()
		if err != nil {
			hour = 4
		}
		return now.Hour() == hour && now.Minute() == 0
	default:
		return false
	}
}
