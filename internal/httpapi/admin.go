package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mushan/coc/internal/account"
)

func (a *API) listAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := a.accounts.List(r.Context())
	if err != nil {
		a.logger.Error("list accounts failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "读取账号失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": accounts})
}

type createAccountRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

func (a *API) createAccount(w http.ResponseWriter, r *http.Request) {
	var request createAccountRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	created, err := a.accounts.Create(
		r.Context(), request.Username, request.DisplayName, request.Password, "user", true,
	)
	if errors.Is(err, account.ErrUsernameTaken) {
		writeError(w, http.StatusConflict, "username_taken", "用户名已经存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_account", "用户名、显示名称和初始密码均不能为空")
		return
	}
	a.recordAudit(r, "account.create", &created.ID, map[string]any{"username": created.Username})
	writeJSON(w, http.StatusCreated, created)
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

func (a *API) resetAccountPassword(w http.ResponseWriter, r *http.Request) {
	var request resetPasswordRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := a.accounts.ResetPassword(r.Context(), r.PathValue("id"), request.Password); err != nil {
		if errors.Is(err, account.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, "account_not_found", "账号不存在")
			return
		}
		writeError(w, http.StatusBadRequest, "empty_password", "新密码不能为空")
		return
	}
	targetID := r.PathValue("id")
	a.recordAudit(r, "account.reset_password", &targetID, nil)
	w.WriteHeader(http.StatusNoContent)
}

type setStatusRequest struct {
	Status string `json:"status"`
}

func (a *API) setAccountStatus(w http.ResponseWriter, r *http.Request) {
	var request setStatusRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	if current.ID == r.PathValue("id") && request.Status == "disabled" {
		writeError(w, http.StatusBadRequest, "cannot_disable_self", "不能停用当前登录的账号")
		return
	}
	if err := a.accounts.SetStatus(r.Context(), r.PathValue("id"), request.Status); err != nil {
		if errors.Is(err, account.ErrLastActiveAdmin) {
			writeError(w, http.StatusConflict, "last_active_admin", "系统必须保留至少一个有效管理员")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_status", "账号状态无效")
		return
	}
	targetID := r.PathValue("id")
	a.recordAudit(r, "account.set_status", &targetID, map[string]any{"status": request.Status})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) revokeAccountSessions(w http.ResponseWriter, r *http.Request) {
	if err := a.accounts.RevokeSessions(r.Context(), r.PathValue("id")); err != nil {
		a.logger.Error("revoke sessions failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "撤销登录失败")
		return
	}
	targetID := r.PathValue("id")
	a.recordAudit(r, "account.revoke_sessions", &targetID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := a.accounts.ListAudit(r.Context(), 100)
	if err != nil {
		a.logger.Error("list audit logs failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "读取审计记录失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": logs})
}

func (a *API) recordAudit(r *http.Request, action string, targetID *string, details any) {
	detailsJSON := []byte("{}")
	if details != nil {
		if encoded, err := json.Marshal(details); err == nil {
			detailsJSON = encoded
		}
	}
	current := currentAccount(r.Context())
	if err := a.accounts.RecordAudit(r.Context(), current.ID, action, targetID, string(detailsJSON)); err != nil {
		a.logger.Error("record admin audit failed", "action", action, "error", err)
	}
}
