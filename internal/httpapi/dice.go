package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mushan/coc/internal/character"
	"github.com/mushan/coc/internal/dice"
	"github.com/mushan/coc/internal/rules/coc7"
)

func (a *API) listRolls(w http.ResponseWriter, r *http.Request) {
	current := currentAccount(r.Context())
	var campaignID *string
	if value := strings.TrimSpace(r.URL.Query().Get("campaignId")); value != "" {
		campaignID = &value
	}
	items, err := a.diceRolls.ListVisible(r.Context(), current.ID, campaignID, 100)
	if err != nil {
		a.logger.Error("list dice rolls failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "读取投骰记录失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type checkRollRequest struct {
	RequestID    string  `json:"requestId"`
	CharacterID  string  `json:"characterId"`
	CampaignID   *string `json:"campaignId"`
	Skill        string  `json:"skill"`
	Attribute    string  `json:"attribute"`
	BonusPenalty int     `json:"bonusPenalty"`
	Visibility   string  `json:"visibility"`
}

func (a *API) rollCheck(w http.ResponseWriter, r *http.Request) {
	var request checkRollRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.RequestID == "" || (strings.TrimSpace(request.Skill) == "" && strings.TrimSpace(request.Attribute) == "") || (request.Skill != "" && request.Attribute != "") || !validRollVisibility(request.Visibility) {
		writeError(w, http.StatusBadRequest, "invalid_roll", "投骰请求无效")
		return
	}
	current := currentAccount(r.Context())
	if request.CampaignID != nil {
		if err := a.campaigns.ValidateRollContext(r.Context(), *request.CampaignID, &request.CharacterID, current.ID); err != nil {
			writeError(w, http.StatusForbidden, "invalid_campaign_context", "人物卡未挂靠到该团本或没有投骰权限")
			return
		}
	}
	item, err := a.characters.GetEditable(r.Context(), request.CharacterID, current.ID)
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取人物卡失败")
		return
	}
	var sheet coc7.Sheet
	if err := json.Unmarshal(item.Sheet, &sheet); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "人物卡数据异常")
		return
	}
	target, label, ok := 0, request.Skill, false
	if request.Skill != "" {
		target, ok = sheet.Skills[request.Skill]
	}
	if request.Attribute != "" {
		label = attributeRollNames[request.Attribute]
		switch request.Attribute {
		case "luck":
			target, ok = sheet.Status.Luck, true
		case "san":
			target, ok = sheet.Status.SAN.Current, true
		default:
			target, ok = sheet.Attributes[request.Attribute]
		}
	}
	if !ok || label == "" {
		writeError(w, http.StatusBadRequest, "check_not_found", "人物卡上没有该检定项目")
		return
	}
	result, err := dice.RollCheck(target, request.BonusPenalty)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_roll", "奖励骰或惩罚骰数量无效")
		return
	}
	roll, err := a.diceRolls.Save(r.Context(), request.RequestID, current.ID, &item.ID, request.CampaignID, request.Visibility, "check", label, "1d100", request, result)
	if errors.Is(err, dice.ErrRequestConflict) {
		writeError(w, http.StatusConflict, "request_id_conflict", "投骰请求编号已被其他操作使用")
		return
	}
	if err != nil {
		a.logger.Error("save check roll failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "保存投骰失败")
		return
	}
	writeJSON(w, http.StatusCreated, roll)
}

type pushCheckRequest struct {
	RequestID      string `json:"requestId"`
	OriginalRollID string `json:"originalRollId"`
}

func (a *API) pushCheck(w http.ResponseWriter, r *http.Request) {
	var request pushCheckRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.RequestID == "" || request.OriginalRollID == "" {
		writeError(w, http.StatusBadRequest, "invalid_push", "孤注一掷请求无效")
		return
	}
	current := currentAccount(r.Context())
	original, requestData, err := a.diceRolls.GetForReroll(r.Context(), request.OriginalRollID, current.ID)
	if err != nil || original.Kind != "check" || original.RerollOfID != nil {
		writeError(w, http.StatusBadRequest, "invalid_push", "该投骰不能进行孤注一掷")
		return
	}
	existingRequestID, alreadyPushed, err := a.diceRolls.RerollRequestID(r.Context(), original.ID)
	if err != nil || (alreadyPushed && existingRequestID != request.RequestID) {
		writeError(w, http.StatusBadRequest, "invalid_push", "该投骰已经进行过孤注一掷")
		return
	}
	var originalResult dice.CheckResult
	var originalRequest checkRollRequest
	if json.Unmarshal(original.Result, &originalResult) != nil || json.Unmarshal(requestData, &originalRequest) != nil || originalResult.Outcome != "failure" || originalRequest.Attribute == "luck" || originalRequest.Attribute == "san" {
		writeError(w, http.StatusBadRequest, "invalid_push", "只有失败的普通技能或属性检定可以孤注一掷")
		return
	}
	result, err := dice.RollCheck(originalResult.Target, originalResult.BonusPenalty)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_push", "原检定数据无效")
		return
	}
	pushRequest := map[string]any{"requestId": request.RequestID, "originalRollId": original.ID}
	roll, err := a.diceRolls.SaveRelated(r.Context(), request.RequestID, current.ID, original.CharacterID, original.CampaignID, original.Visibility, "check", "孤注一掷 · "+original.Label, "1d100", pushRequest, result, original.ID, "push")
	if errors.Is(err, dice.ErrRequestConflict) {
		writeError(w, http.StatusConflict, "request_id_conflict", "投骰请求编号已被其他操作使用")
		return
	}
	if err != nil {
		a.logger.Error("save pushed roll failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "保存孤注一掷失败")
		return
	}
	writeJSON(w, http.StatusCreated, roll)
}

type rerollRequest struct {
	RequestID      string `json:"requestId"`
	OriginalRollID string `json:"originalRollId"`
}

func (a *API) reroll(w http.ResponseWriter, r *http.Request) {
	var request rerollRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.RequestID == "" || request.OriginalRollID == "" {
		writeError(w, http.StatusBadRequest, "invalid_reroll", "重投请求无效")
		return
	}
	current := currentAccount(r.Context())
	original, _, err := a.diceRolls.GetForReroll(r.Context(), request.OriginalRollID, current.ID)
	if err != nil || (original.Kind != "check" && original.Kind != "expression") {
		writeError(w, http.StatusBadRequest, "invalid_reroll", "只能重投自己的检定或组合骰")
		return
	}
	if original.CampaignID != nil {
		if err := a.campaigns.ValidateRollContext(r.Context(), *original.CampaignID, original.CharacterID, current.ID); err != nil {
			writeError(w, http.StatusForbidden, "invalid_campaign_context", "当前已没有该团本的投骰权限")
			return
		}
	}
	requestData := map[string]any{"requestId": request.RequestID, "originalRollId": original.ID}
	var result any
	if original.Kind == "check" {
		var prior dice.CheckResult
		if json.Unmarshal(original.Result, &prior) != nil {
			writeError(w, http.StatusBadRequest, "invalid_reroll", "原检定结果无效")
			return
		}
		result, err = dice.RollCheck(prior.Target, prior.BonusPenalty)
	} else {
		result, err = dice.RollExpression(original.Expression)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_reroll", "原投骰参数无效")
		return
	}
	roll, err := a.diceRolls.SaveRelated(r.Context(), request.RequestID, current.ID, original.CharacterID, original.CampaignID, original.Visibility, original.Kind, "重投 · "+original.Label, original.Expression, requestData, result, original.ID, "reroll")
	if errors.Is(err, dice.ErrRequestConflict) {
		writeError(w, http.StatusConflict, "request_id_conflict", "投骰请求编号已被其他操作使用")
		return
	}
	if err != nil {
		a.logger.Error("save reroll failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "保存重投失败")
		return
	}
	writeJSON(w, http.StatusCreated, roll)
}

var attributeRollNames = map[string]string{"str": "力量", "con": "体质", "siz": "体型", "dex": "敏捷", "app": "外貌", "int": "智力", "pow": "意志", "edu": "教育", "luck": "幸运", "san": "理智"}

type expressionRollRequest struct {
	RequestID   string  `json:"requestId"`
	CharacterID *string `json:"characterId"`
	CampaignID  *string `json:"campaignId"`
	Expression  string  `json:"expression"`
	Label       string  `json:"label"`
	Visibility  string  `json:"visibility"`
}

func (a *API) rollExpression(w http.ResponseWriter, r *http.Request) {
	var request expressionRollRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.RequestID == "" || !validRollVisibility(request.Visibility) {
		writeError(w, http.StatusBadRequest, "invalid_roll", "投骰请求无效")
		return
	}
	current := currentAccount(r.Context())
	if request.CampaignID != nil {
		if err := a.campaigns.ValidateRollContext(r.Context(), *request.CampaignID, request.CharacterID, current.ID); err != nil {
			writeError(w, http.StatusForbidden, "invalid_campaign_context", "人物卡未挂靠到该团本或没有投骰权限")
			return
		}
	}
	if request.CharacterID != nil {
		if _, err := a.characters.GetEditable(r.Context(), *request.CharacterID, current.ID); err != nil {
			writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
			return
		}
	}
	result, err := dice.RollExpression(request.Expression)
	if errors.Is(err, dice.ErrInvalidExpression) {
		writeError(w, http.StatusBadRequest, "invalid_expression", "骰子表达式无效")
		return
	}
	label := strings.TrimSpace(request.Label)
	if label == "" {
		label = request.Expression
	}
	roll, err := a.diceRolls.Save(r.Context(), request.RequestID, current.ID, request.CharacterID, request.CampaignID, request.Visibility, "expression", label, request.Expression, request, result)
	if errors.Is(err, dice.ErrRequestConflict) {
		writeError(w, http.StatusConflict, "request_id_conflict", "投骰请求编号已被其他操作使用")
		return
	}
	if err != nil {
		a.logger.Error("save expression roll failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "保存投骰失败")
		return
	}
	writeJSON(w, http.StatusCreated, roll)
}

func validRollVisibility(value string) bool {
	return value == "public" || value == "keeper" || value == "test"
}
