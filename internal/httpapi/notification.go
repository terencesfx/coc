package httpapi

import (
	"errors"
	"net/http"

	"github.com/mushan/coc/internal/notification"
)

func (a *API) getCampaignNotifications(w http.ResponseWriter, r *http.Request) {
	setting, canManage, err := a.notifications.GetSetting(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "campaign_not_found", "团本不存在")
		return
	}
	if !canManage {
		writeError(w, http.StatusForbidden, "keeper_required", "只有本团 KP 可以查看通知设置")
		return
	}
	writeJSON(w, http.StatusOK, setting)
}

func (a *API) setCampaignNotifications(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Provider        string `json:"provider"`
		TargetReference string `json:"targetReference"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	setting, err := a.notifications.SetSetting(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID, request.Provider, request.TargetReference)
	if errors.Is(err, notification.ErrForbidden) {
		writeError(w, http.StatusForbidden, "keeper_required", "只有本团 KP 可以修改通知设置")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_notification_setting", "通知设置无效")
		return
	}
	writeJSON(w, http.StatusOK, setting)
}

func (a *API) listNotificationDeliveries(w http.ResponseWriter, r *http.Request) {
	items, err := a.notifications.ListDeliveries(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if errors.Is(err, notification.ErrForbidden) {
		writeError(w, http.StatusForbidden, "keeper_required", "只有本团 KP 可以查看投递记录")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取投递记录失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
