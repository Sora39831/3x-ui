package controller

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// BackupController handles database backup and restore API endpoints.
type BackupController struct {
	BaseController
	backupService service.BackupService
}

// initRouter registers backup API routes.
func (a *BackupController) initRouter(g *gin.RouterGroup) {
	g.POST("/backup", a.createBackup)
	g.POST("/restore/:filename", a.restoreBackup)
	g.POST("/deleteBackup/:filename", a.deleteBackup)
	g.GET("/listBackups", a.listBackups)
	g.GET("/downloadBackup/:filename", a.downloadBackup)
}

// createBackup creates an immediate manual backup.
func (a *BackupController) createBackup(c *gin.Context) {
	filePath, err := a.backupService.CreateBackup()
	if err != nil {
		jsonMsg(c, "create backup failed", err)
		return
	}
	jsonObj(c, filePath, nil)
}

// restoreBackup restores the database from a backup file.
func (a *BackupController) restoreBackup(c *gin.Context) {
	filename := c.Param("filename")
	if err := a.backupService.RestoreBackup(filename); err != nil {
		jsonMsg(c, "restore backup failed", err)
		return
	}
	jsonObj(c, "restore completed", nil)
}

// deleteBackup deletes a backup file.
func (a *BackupController) deleteBackup(c *gin.Context) {
	filename := c.Param("filename")
	if err := a.backupService.DeleteBackup(filename); err != nil {
		jsonMsg(c, "delete failed", err)
		return
	}
	jsonObj(c, "deleted", nil)
}

// listBackups lists all backup files.
func (a *BackupController) listBackups(c *gin.Context) {
	entries, err := a.backupService.ListBackups()
	if err != nil {
		jsonMsg(c, "list backups failed", err)
		return
	}
	jsonObj(c, entries, nil)
}

// downloadBackup downloads a backup file.
func (a *BackupController) downloadBackup(c *gin.Context) {
	filename := c.Param("filename")
	filePath := a.backupService.GetBackupPath(filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		jsonMsg(c, "read backup file failed", err)
		return
	}
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Writer.Write(data)
}
