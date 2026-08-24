package httpapi

import (
	"net/http"
	"strings"
)

func (a *API) systemStatus(w http.ResponseWriter, r *http.Request) {
	status, err := a.maintenance.Status(r.Context())
	if err != nil {
		a.logger.Error("read system status failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "读取系统状态失败")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *API) listBackups(w http.ResponseWriter, _ *http.Request) {
	backups, err := a.maintenance.ListBackups()
	if err != nil {
		a.logger.Error("list backups failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "读取备份失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": backups})
}

func (a *API) createBackup(w http.ResponseWriter, r *http.Request) {
	backup, err := a.maintenance.CreateBackup(r.Context())
	if err != nil {
		a.logger.Error("create backup failed", "error", err)
		writeError(w, http.StatusInternalServerError, "backup_failed", "创建数据库备份失败")
		return
	}
	a.recordAudit(r, "backup.create", nil, map[string]any{"name": backup.Name, "sizeBytes": backup.SizeBytes})
	writeJSON(w, http.StatusCreated, backup)
}

func (a *API) downloadBackup(w http.ResponseWriter, r *http.Request) {
	path, err := a.maintenance.BackupPath(r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "backup_not_found", "备份不存在")
		return
	}
	a.recordAudit(r, "backup.download", nil, map[string]any{"name": r.PathValue("name")})
	w.Header().Set("Content-Disposition", `attachment; filename="`+r.PathValue("name")+`"`)
	if strings.HasSuffix(r.PathValue("name"), ".db") {
		w.Header().Set("Content-Type", "application/vnd.sqlite3")
	} else {
		w.Header().Set("Content-Type", "application/gzip")
	}
	http.ServeFile(w, r, path)
}

func (a *API) validateBackup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, (1<<30)+(1<<20))
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_backup", "备份包过大或上传格式错误")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_backup", "请选择备份包")
		return
	}
	defer file.Close()
	report, err := a.maintenance.ValidateUpload(file)
	if err != nil {
		a.logger.Warn("backup validation failed", "error", err)
		writeError(w, http.StatusBadRequest, "invalid_backup", "备份包校验失败")
		return
	}
	a.recordAudit(r, "backup.validate", nil, map[string]any{"fileCount": report.FileCount, "assetCount": report.AssetCount})
	writeJSON(w, http.StatusOK, report)
}
