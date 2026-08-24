package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/mushan/coc/internal/account"
	"github.com/mushan/coc/internal/campaign"
	"github.com/mushan/coc/internal/character"
	"github.com/mushan/coc/internal/dice"
	"github.com/mushan/coc/internal/maintenance"
	"github.com/mushan/coc/internal/notification"
	"github.com/mushan/coc/internal/rules/coc7"
)

type API struct {
	logger        *slog.Logger
	startedAt     time.Time
	accounts      *account.Store
	characters    *character.Store
	campaigns     *campaign.Store
	diceRolls     *dice.Store
	occupations   *coc7.OccupationCatalog
	maintenance   *maintenance.Service
	notifications *notification.Store
	cookieSecure  bool
}

func New(logger *slog.Logger, accounts *account.Store, characters *character.Store, campaigns *campaign.Store, diceRolls *dice.Store, occupations *coc7.OccupationCatalog, maintenanceService *maintenance.Service, notifications *notification.Store, cookieSecure bool) http.Handler {
	api := &API{
		logger: logger, startedAt: time.Now().UTC(), accounts: accounts, characters: characters, campaigns: campaigns, diceRolls: diceRolls, occupations: occupations, maintenance: maintenanceService, notifications: notifications, cookieSecure: cookieSecure,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health/live", api.live)
	mux.HandleFunc("GET /api/v1/health/ready", api.ready)
	mux.HandleFunc("POST /api/v1/auth/login", api.login)
	mux.Handle("POST /api/v1/auth/logout", api.requireAuth(http.HandlerFunc(api.logout)))
	mux.Handle("GET /api/v1/auth/me", api.requireAuth(http.HandlerFunc(api.me)))
	mux.Handle("PUT /api/v1/auth/password", api.requireAuth(http.HandlerFunc(api.changePassword)))
	mux.Handle("GET /api/v1/admin/accounts", api.requireAdmin(http.HandlerFunc(api.listAccounts)))
	mux.Handle("POST /api/v1/admin/accounts", api.requireAdmin(http.HandlerFunc(api.createAccount)))
	mux.Handle("POST /api/v1/admin/accounts/{id}/reset-password", api.requireAdmin(http.HandlerFunc(api.resetAccountPassword)))
	mux.Handle("PATCH /api/v1/admin/accounts/{id}/status", api.requireAdmin(http.HandlerFunc(api.setAccountStatus)))
	mux.Handle("POST /api/v1/admin/accounts/{id}/revoke-sessions", api.requireAdmin(http.HandlerFunc(api.revokeAccountSessions)))
	mux.Handle("GET /api/v1/admin/audit-logs", api.requireAdmin(http.HandlerFunc(api.listAuditLogs)))
	mux.Handle("GET /api/v1/admin/system/status", api.requireAdmin(http.HandlerFunc(api.systemStatus)))
	mux.Handle("GET /api/v1/admin/backups", api.requireAdmin(http.HandlerFunc(api.listBackups)))
	mux.Handle("POST /api/v1/admin/backups", api.requireAdmin(http.HandlerFunc(api.createBackup)))
	mux.Handle("GET /api/v1/admin/backups/{name}", api.requireAdmin(http.HandlerFunc(api.downloadBackup)))
	mux.Handle("POST /api/v1/admin/backups/validate", api.requireAdmin(http.HandlerFunc(api.validateBackup)))
	mux.Handle("GET /api/v1/characters", api.requireAuth(http.HandlerFunc(api.listCharacters)))
	mux.Handle("POST /api/v1/characters", api.requireAuth(http.HandlerFunc(api.createCharacter)))
	mux.Handle("GET /api/v1/characters/{id}", api.requireAuth(http.HandlerFunc(api.getCharacter)))
	mux.Handle("DELETE /api/v1/characters/{id}", api.requireAuth(http.HandlerFunc(api.deleteCharacter)))
	mux.Handle("GET /api/v1/characters/{id}/campaigns", api.requireAuth(http.HandlerFunc(api.listCharacterCampaigns)))
	mux.Handle("PATCH /api/v1/characters/{id}", api.requireAuth(http.HandlerFunc(api.updateCharacter)))
	mux.Handle("GET /api/v1/characters/{id}/versions", api.requireAuth(http.HandlerFunc(api.listCharacterVersions)))
	mux.Handle("GET /api/v1/characters/{id}/versions/{version}", api.requireAuth(http.HandlerFunc(api.getCharacterVersion)))
	mux.Handle("GET /api/v1/characters/{id}/compare", api.requireAuth(http.HandlerFunc(api.compareCharacterVersions)))
	mux.Handle("POST /api/v1/characters/{id}/restore/{version}", api.requireAuth(http.HandlerFunc(api.restoreCharacter)))
	mux.Handle("POST /api/v1/characters/{id}/generate-attributes", api.requireAuth(http.HandlerFunc(api.generateCharacterAttributes)))
	mux.Handle("POST /api/v1/characters/{id}/age-adjustment", api.requireAuth(http.HandlerFunc(api.applyCharacterAgeAdjustment)))
	mux.Handle("GET /api/v1/rules/coc7/occupations", api.requireAuth(http.HandlerFunc(api.listOccupations)))
	mux.Handle("POST /api/v1/characters/{id}/occupation", api.requireAuth(http.HandlerFunc(api.applyCharacterOccupation)))
	mux.Handle("PUT /api/v1/characters/{id}/skill-allocation", api.requireAuth(http.HandlerFunc(api.allocateCharacterSkills)))
	mux.Handle("POST /api/v1/characters/{id}/skill-growth", api.requireAuth(http.HandlerFunc(api.growCharacterSkills)))
	mux.Handle("POST /api/v1/characters/{id}/copy", api.requireAuth(http.HandlerFunc(api.copyCharacter)))
	mux.Handle("PATCH /api/v1/characters/{id}/status", api.requireAuth(http.HandlerFunc(api.setCharacterStatus)))
	mux.Handle("POST /api/v1/rolls/check", api.requireAuth(http.HandlerFunc(api.rollCheck)))
	mux.Handle("POST /api/v1/rolls/push", api.requireAuth(http.HandlerFunc(api.pushCheck)))
	mux.Handle("POST /api/v1/rolls/reroll", api.requireAuth(http.HandlerFunc(api.reroll)))
	mux.Handle("POST /api/v1/rolls/expression", api.requireAuth(http.HandlerFunc(api.rollExpression)))
	mux.Handle("GET /api/v1/rolls", api.requireAuth(http.HandlerFunc(api.listRolls)))
	mux.Handle("GET /api/v1/campaigns", api.requireAuth(http.HandlerFunc(api.listCampaigns)))
	mux.Handle("POST /api/v1/campaigns", api.requireAuth(http.HandlerFunc(api.createCampaign)))
	mux.Handle("GET /api/v1/campaigns/{id}", api.requireAuth(http.HandlerFunc(api.getCampaign)))
	mux.Handle("PATCH /api/v1/campaigns/{id}", api.requireAuth(http.HandlerFunc(api.updateCampaign)))
	mux.Handle("DELETE /api/v1/campaigns/{id}", api.requireAuth(http.HandlerFunc(api.deleteCampaign)))
	mux.Handle("PATCH /api/v1/campaigns/{id}/cover", api.requireAuth(http.HandlerFunc(api.setCampaignCover)))
	mux.Handle("GET /api/v1/campaigns/{id}/blocks", api.requireAuth(http.HandlerFunc(api.listCampaignBlocks)))
	mux.Handle("POST /api/v1/campaigns/{id}/blocks", api.requireAuth(http.HandlerFunc(api.createCampaignBlock)))
	mux.Handle("PATCH /api/v1/campaigns/{id}/blocks/{blockID}", api.requireAuth(http.HandlerFunc(api.updateCampaignBlock)))
	mux.Handle("DELETE /api/v1/campaigns/{id}/blocks/{blockID}", api.requireAuth(http.HandlerFunc(api.deleteCampaignBlock)))
	mux.Handle("POST /api/v1/campaigns/{id}/blocks/{blockID}/move", api.requireAuth(http.HandlerFunc(api.moveCampaignBlock)))
	mux.Handle("POST /api/v1/campaigns/{id}/assets", api.requireAuth(http.HandlerFunc(api.uploadCampaignAsset)))
	mux.Handle("GET /api/v1/campaigns/{id}/assets/{assetID}", api.requireAuth(http.HandlerFunc(api.getCampaignAsset)))
	mux.Handle("GET /api/v1/campaigns/{id}/characters", api.requireAuth(http.HandlerFunc(api.listCampaignCharacters)))
	mux.Handle("GET /api/v1/campaigns/{id}/rolls", api.requireAuth(http.HandlerFunc(api.listCampaignRolls)))
	mux.Handle("POST /api/v1/campaigns/{id}/characters", api.requireAuth(http.HandlerFunc(api.attachCampaignCharacter)))
	mux.Handle("PATCH /api/v1/campaigns/{id}/characters/{characterID}", api.requireAuth(http.HandlerFunc(api.setCampaignCharacterVisibility)))
	mux.Handle("DELETE /api/v1/campaigns/{id}/characters/{characterID}", api.requireAuth(http.HandlerFunc(api.detachCampaignCharacter)))
	mux.Handle("GET /api/v1/campaigns/{id}/notifications", api.requireAuth(http.HandlerFunc(api.getCampaignNotifications)))
	mux.Handle("PUT /api/v1/campaigns/{id}/notifications", api.requireAuth(http.HandlerFunc(api.setCampaignNotifications)))
	mux.Handle("GET /api/v1/campaigns/{id}/notification-deliveries", api.requireAuth(http.HandlerFunc(api.listNotificationDeliveries)))

	return api.requestID(api.recoverPanic(api.logRequest(api.securityHeaders(mux))))
}

func (a *API) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"startedAt": a.startedAt,
	})
}

func (a *API) ready(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求内容格式错误")
		return false
	}
	return true
}
