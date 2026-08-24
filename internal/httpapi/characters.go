package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mushan/coc/internal/character"
	"github.com/mushan/coc/internal/rules/coc7"
)

func (a *API) listCharacters(w http.ResponseWriter, r *http.Request) {
	items, err := a.characters.ListOwned(r.Context(), currentAccount(r.Context()).ID)
	if err != nil {
		a.characterInternalError(w, "list characters", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type createCharacterRequest struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func (a *API) createCharacter(w http.ResponseWriter, r *http.Request) {
	var request createCharacterRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	created, err := a.characters.Create(r.Context(), current.ID, current.ID, request.Name, request.Kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_character", "请填写人物姓名并选择人物类型")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) copyCharacter(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	item, err := a.characters.CopyOwned(r.Context(), r.PathValue("id"), current.ID, current.ID, request.Name)
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "character_not_found", "只能复制自己的人物卡")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_character", "复制人物卡失败")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) setCharacterStatus(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	item, err := a.characters.SetStatusOwned(r.Context(), r.PathValue("id"), current.ID, current.ID, request.Status)
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusForbidden, "owner_required", "只有人物卡所有者可以修改状态")
		return
	}
	if errors.Is(err, character.ErrConflict) {
		writeError(w, http.StatusConflict, "version_conflict", "人物卡已被其他操作修改")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_status", "人物状态无效")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) getCharacter(w http.ResponseWriter, r *http.Request) {
	item, err := a.characters.GetVisible(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
		return
	}
	if err != nil {
		a.characterInternalError(w, "get character", err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) deleteCharacter(w http.ResponseWriter, r *http.Request) {
	err := a.characters.ArchiveOwned(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在或不属于当前账户")
		return
	}
	if err != nil {
		a.characterInternalError(w, "archive character", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type updateCharacterRequest struct {
	BaseVersion int             `json:"baseVersion"`
	Sheet       json.RawMessage `json:"sheet"`
	Message     string          `json:"message"`
}

func (a *API) updateCharacter(w http.ResponseWriter, r *http.Request) {
	var request updateCharacterRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	editable, err := a.characters.GetEditable(r.Context(), r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在或不可编辑")
		return
	}
	item, err := a.characters.Update(r.Context(), r.PathValue("id"), editable.OwnerAccountID, current.ID, request.BaseVersion, request.Sheet, request.Message)
	if errors.Is(err, character.ErrConflict) {
		writeError(w, http.StatusConflict, "version_conflict", "人物卡已被其他操作修改，请刷新后重试")
		return
	}
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_sheet", "人物卡数据格式错误")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) listCharacterVersions(w http.ResponseWriter, r *http.Request) {
	visible, accessErr := a.characters.GetVisible(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if accessErr != nil {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
		return
	}
	items, err := a.characters.Versions(r.Context(), r.PathValue("id"), visible.OwnerAccountID)
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
		return
	}
	if err != nil {
		a.characterInternalError(w, "list character versions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) getCharacterVersion(w http.ResponseWriter, r *http.Request) {
	to, err := strconv.Atoi(r.PathValue("version"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_version", "版本号无效")
		return
	}
	visible, err := a.characters.GetVisible(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
		return
	}
	var from *int
	if to > 1 {
		value := to - 1
		from = &value
	}
	detail, err := a.characters.VersionDetail(r.Context(), visible.ID, visible.OwnerAccountID, from, to)
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "version_not_found", "人物卡版本不存在")
		return
	}
	if err != nil {
		a.characterInternalError(w, "get character version", err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (a *API) compareCharacterVersions(w http.ResponseWriter, r *http.Request) {
	from, errFrom := strconv.Atoi(r.URL.Query().Get("from"))
	to, errTo := strconv.Atoi(r.URL.Query().Get("to"))
	if errFrom != nil || errTo != nil || from < 1 || to < 1 || from == to {
		writeError(w, http.StatusBadRequest, "invalid_version", "请选择两个不同版本")
		return
	}
	visible, err := a.characters.GetVisible(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
		return
	}
	detail, err := a.characters.VersionDetail(r.Context(), visible.ID, visible.OwnerAccountID, &from, to)
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "version_not_found", "人物卡版本不存在")
		return
	}
	if err != nil {
		a.characterInternalError(w, "compare character versions", err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

type restoreCharacterRequest struct {
	BaseVersion int    `json:"baseVersion"`
	Message     string `json:"message"`
}

func (a *API) restoreCharacter(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_version", "版本号无效")
		return
	}
	var request restoreCharacterRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	editable, err := a.characters.GetEditable(r.Context(), r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在或不可编辑")
		return
	}
	item, err := a.characters.Restore(r.Context(), r.PathValue("id"), editable.OwnerAccountID, current.ID, version, request.BaseVersion, request.Message)
	if errors.Is(err, character.ErrConflict) {
		writeError(w, http.StatusConflict, "version_conflict", "人物卡已被修改，请刷新后重试")
		return
	}
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "version_not_found", "人物卡版本不存在")
		return
	}
	if err != nil {
		a.characterInternalError(w, "restore character", err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

type generateAttributesRequest struct {
	BaseVersion int `json:"baseVersion"`
}

func (a *API) generateCharacterAttributes(w http.ResponseWriter, r *http.Request) {
	var request generateAttributesRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	editable, err := a.characters.GetEditable(r.Context(), r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在或不可编辑")
		return
	}
	item, err := a.characters.GenerateAttributes(r.Context(), r.PathValue("id"), editable.OwnerAccountID, current.ID, request.BaseVersion)
	if errors.Is(err, character.ErrConflict) {
		writeError(w, http.StatusConflict, "version_conflict", "人物卡已被修改，请刷新后重试")
		return
	}
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
		return
	}
	if err != nil {
		a.characterInternalError(w, "generate character attributes", err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

type applyOccupationRequest struct {
	BaseVersion  int    `json:"baseVersion"`
	OccupationID string `json:"occupationId"`
	FormulaIndex int    `json:"formulaIndex"`
}

type applyAgeAdjustmentRequest struct {
	BaseVersion int            `json:"baseVersion"`
	Reductions  map[string]int `json:"reductions"`
}

func (a *API) applyCharacterAgeAdjustment(w http.ResponseWriter, r *http.Request) {
	var request applyAgeAdjustmentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	editable, err := a.characters.GetEditable(r.Context(), r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在或不可编辑")
		return
	}
	item, result, err := a.characters.ApplyAgeAdjustment(r.Context(), r.PathValue("id"), editable.OwnerAccountID, current.ID, request.BaseVersion, request.Reductions)
	if errors.Is(err, character.ErrConflict) {
		writeError(w, http.StatusConflict, "version_conflict", "人物卡已被修改，请刷新后重试")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_age_adjustment", "年龄修正分配无效，或已经应用过年龄修正")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"character": item, "result": result})
}

func (a *API) applyCharacterOccupation(w http.ResponseWriter, r *http.Request) {
	var request applyOccupationRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if a.occupations == nil {
		writeError(w, http.StatusBadRequest, "occupation_catalog_unavailable", "职业目录不可用")
		return
	}
	occupation, ok := a.occupations.Find(request.OccupationID)
	if !ok {
		writeError(w, http.StatusBadRequest, "occupation_not_found", "职业不存在")
		return
	}
	current := currentAccount(r.Context())
	editable, err := a.characters.GetEditable(r.Context(), r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在或不可编辑")
		return
	}
	item, err := a.characters.ApplyOccupation(r.Context(), r.PathValue("id"), editable.OwnerAccountID, current.ID, request.BaseVersion, occupation, request.FormulaIndex)
	if errors.Is(err, character.ErrConflict) {
		writeError(w, http.StatusConflict, "version_conflict", "人物卡已被修改，请刷新后重试")
		return
	}
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_occupation", "职业公式或人物属性无效")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

type allocateSkillsRequest struct {
	BaseVersion int                  `json:"baseVersion"`
	Allocation  coc7.SkillAllocation `json:"allocation"`
}

func (a *API) allocateCharacterSkills(w http.ResponseWriter, r *http.Request) {
	var request allocateSkillsRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	editable, err := a.characters.GetEditable(r.Context(), r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在或不可编辑")
		return
	}
	item, err := a.characters.AllocateSkills(r.Context(), r.PathValue("id"), editable.OwnerAccountID, current.ID, request.BaseVersion, request.Allocation)
	if errors.Is(err, character.ErrConflict) {
		writeError(w, http.StatusConflict, "version_conflict", "人物卡已被修改，请刷新后重试")
		return
	}
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在")
		return
	}
	if err != nil {
		a.logger.Warn("skill allocation rejected", "character_id", r.PathValue("id"), "error", err)
		writeError(w, http.StatusBadRequest, "invalid_skill_allocation", skillAllocationErrorMessage(err))
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func skillAllocationErrorMessage(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "choices are incomplete"), strings.Contains(message, "requires"):
		return "请完整选择职业要求的技能"
	case strings.Contains(message, "invalid occupation skill choice"), strings.Contains(message, "invalid free skill choice"):
		return "职业技能不能重复选择，也不能选择克苏鲁神话"
	case strings.Contains(message, "not in choice group"):
		return "选择的技能不属于该职业技能组"
	case strings.Contains(message, "unknown skill"):
		return "选择的职业技能尚未添加到人物卡"
	case strings.Contains(message, "budget exceeded"):
		return "分配的技能点超过了可用点数"
	case strings.Contains(message, "exceeds 99"):
		return "技能最终值不能超过 99"
	case strings.Contains(message, "credit rating must be between"):
		var minimum, maximum int
		if _, scanErr := fmt.Sscanf(message, "credit rating must be between %d and %d", &minimum, &maximum); scanErr == nil {
			return fmt.Sprintf("信用评级最终值必须在 %d～%d 之间", minimum, maximum)
		}
		return "信用评级不符合职业要求"
	default:
		return "技能分配不符合职业规则"
	}
}

type growSkillsRequest struct {
	BaseVersion int      `json:"baseVersion"`
	Skills      []string `json:"skills"`
}

func (a *API) growCharacterSkills(w http.ResponseWriter, r *http.Request) {
	var request growSkillsRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	editable, err := a.characters.GetEditable(r.Context(), r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "character_not_found", "人物卡不存在或不可编辑")
		return
	}
	item, result, err := a.characters.GrowSkills(r.Context(), r.PathValue("id"), editable.OwnerAccountID, current.ID, request.BaseVersion, request.Skills)
	if errors.Is(err, character.ErrConflict) {
		writeError(w, http.StatusConflict, "version_conflict", "人物卡已被修改，请刷新后重试")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_skill_growth", "成长技能选择无效")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"character": item, "result": result})
}

func (a *API) characterInternalError(w http.ResponseWriter, operation string, err error) {
	a.logger.Error(operation+" failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "人物卡操作失败")
}
