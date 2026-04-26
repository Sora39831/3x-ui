package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// BackupJob handles scheduled database backups.
type BackupJob struct {
	settingService service.SettingService
	backupService  service.BackupService
}

// NewBackupJob creates a new BackupJob instance.
func NewBackupJob() *BackupJob {
	return &BackupJob{}
}

// Run executes the scheduled backup if enabled and time matches the configured frequency.
func (j *BackupJob) Run() {
	enabled, err := j.settingService.GetBackupEnabled()
	if err != nil || !enabled {
		return
	}

	frequency, err := j.settingService.GetBackupFrequency()
	if err != nil {
		return
	}

	if !j.shouldRun(frequency) {
		return
	}

	if err := j.backupService.CreateSnapshot(j.settingService); err != nil {
		logger.Warning("scheduled backup failed:", err)
	}
}

// shouldRun checks if the backup should run based on the configured frequency.
func (j *BackupJob) shouldRun(frequency string) bool {
	now := time.Now()

	switch frequency {
	case "hourly":
		return now.Minute() == 0
	case "every12h":
		return (now.Hour() == 0 || now.Hour() == 12) && now.Minute() == 0
	case "daily":
		hour, err := j.settingService.GetBackupHour()
		if err != nil {
			hour = 3
		}
		return now.Hour() == hour && now.Minute() == 0
	case "weekly":
		if now.Weekday() != time.Sunday {
			return false
		}
		hour, err := j.settingService.GetBackupHour()
		if err != nil {
			hour = 3
		}
		return now.Hour() == hour && now.Minute() == 0
	default:
		return false
	}
}
