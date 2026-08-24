package httpapi

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"github.com/mushan/coc/internal/campaign"
	_ "golang.org/x/image/webp"
)

func (a *API) listCampaigns(w http.ResponseWriter, r *http.Request) {
	items, err := a.campaigns.List(r.Context(), currentAccount(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取团本失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) listCharacterCampaigns(w http.ResponseWriter, r *http.Request) {
	items, err := a.campaigns.ListForCharacter(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取人物卡关联团本失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type campaignRequest struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

func (a *API) createCampaign(w http.ResponseWriter, r *http.Request) {
	var request campaignRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := a.campaigns.Create(r.Context(), currentAccount(r.Context()).ID, request.Title, request.Summary)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_campaign", "团本名称或简介无效")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) getCampaign(w http.ResponseWriter, r *http.Request) {
	item, err := a.campaigns.Get(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign_not_found", "团本不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取团本失败")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) updateCampaign(w http.ResponseWriter, r *http.Request) {
	var request campaignRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := a.campaigns.Update(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID, request.Title, request.Summary, request.Status)
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign_not_found", "团本不存在")
		return
	}
	if errors.Is(err, campaign.ErrForbidden) {
		writeError(w, http.StatusForbidden, "keeper_required", "只有本团 KP 可以修改")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_campaign", "团本资料无效")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) deleteCampaign(w http.ResponseWriter, r *http.Request) {
	err := a.campaigns.Archive(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if errors.Is(err, campaign.ErrForbidden) {
		writeError(w, http.StatusForbidden, "keeper_required", "只有本团 KP 可以删除团本")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "删除团本失败")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) setCampaignCover(w http.ResponseWriter, r *http.Request) {
	var request struct {
		AssetID *string `json:"assetId"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := a.campaigns.SetCover(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID, request.AssetID)
	if errors.Is(err, campaign.ErrForbidden) {
		writeError(w, http.StatusForbidden, "keeper_required", "只有本团 KP 可以修改封面")
		return
	}
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "invalid_asset", "封面图片不属于本团")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "设置封面失败")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

type blockRequest struct {
	Type       string  `json:"type"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Visibility string  `json:"visibility"`
	AssetID    *string `json:"assetId"`
}

func (a *API) listCampaignBlocks(w http.ResponseWriter, r *http.Request) {
	items, err := a.campaigns.ListBlocks(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign_not_found", "团本不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取内容失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) createCampaignBlock(w http.ResponseWriter, r *http.Request) {
	var request blockRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := a.campaigns.CreateBlock(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID, request.Type, request.Title, request.Content, request.Visibility, request.AssetID)
	writeBlockResult(w, item, err, http.StatusCreated)
}

func (a *API) updateCampaignBlock(w http.ResponseWriter, r *http.Request) {
	var request blockRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := a.campaigns.UpdateBlock(r.Context(), r.PathValue("id"), r.PathValue("blockID"), currentAccount(r.Context()).ID, request.Type, request.Title, request.Content, request.Visibility, request.AssetID)
	writeBlockResult(w, item, err, http.StatusOK)
}

func (a *API) uploadCampaignAsset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_image", "图片不能超过 10 MB")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_image", "请选择图片")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 10<<20 {
		writeError(w, http.StatusBadRequest, "invalid_image", "图片不能超过 10 MB")
		return
	}
	mimeType, extension, width, height, ok := inspectImage(data)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_image", "只支持 JPEG、PNG 和 WebP 图片")
		return
	}
	asset, err := a.campaigns.SaveAsset(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID, header.Filename, mimeType, extension, data, width, height)
	if errors.Is(err, campaign.ErrForbidden) {
		writeError(w, http.StatusForbidden, "keeper_required", "只有本团 KP 可以上传图片")
		return
	}
	if err != nil {
		a.logger.Error("save campaign image failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "保存图片失败")
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

func (a *API) getCampaignAsset(w http.ResponseWriter, r *http.Request) {
	asset, err := a.campaigns.GetVisibleAsset(r.Context(), r.PathValue("id"), r.PathValue("assetID"), currentAccount(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "asset_not_found", "图片不存在或不可见")
		return
	}
	w.Header().Set("Content-Type", asset.MimeType)
	w.Header().Set("Content-Disposition", `inline; filename="image"`)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, a.campaigns.AssetPath(asset))
}

func inspectImage(data []byte) (string, string, *int, *int, bool) {
	detected := http.DetectContentType(data)
	if detected != "image/jpeg" && detected != "image/png" && detected != "image/webp" {
		return "", "", nil, nil, false
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") || config.Width < 1 || config.Height < 1 {
		return "", "", nil, nil, false
	}
	extension := "." + strings.Replace(format, "jpeg", "jpg", 1)
	return detected, extension, &config.Width, &config.Height, true
}

func (a *API) deleteCampaignBlock(w http.ResponseWriter, r *http.Request) {
	err := a.campaigns.DeleteBlock(r.Context(), r.PathValue("id"), r.PathValue("blockID"), currentAccount(r.Context()).ID)
	if !writeCampaignMutationError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) moveCampaignBlock(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Direction string `json:"direction"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	err := a.campaigns.MoveBlock(r.Context(), r.PathValue("id"), r.PathValue("blockID"), currentAccount(r.Context()).ID, request.Direction)
	if !writeCampaignMutationError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeBlockResult(w http.ResponseWriter, item campaign.Block, err error, status int) {
	if !writeCampaignMutationError(w, err) {
		return
	}
	writeJSON(w, status, item)
}

func writeCampaignMutationError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, campaign.ErrForbidden) {
		writeError(w, http.StatusForbidden, "keeper_required", "只有本团 KP 可以修改")
		return false
	}
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "团本或内容块不存在")
		return false
	}
	writeError(w, http.StatusBadRequest, "invalid_block", "内容块数据无效")
	return false
}

func (a *API) listCampaignCharacters(w http.ResponseWriter, r *http.Request) {
	items, err := a.campaigns.ListCharacters(r.Context(), r.PathValue("id"), currentAccount(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取团本人物失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) listCampaignRolls(w http.ResponseWriter, r *http.Request) {
	current := currentAccount(r.Context())
	campaignID := r.PathValue("id")
	if _, err := a.campaigns.Get(r.Context(), campaignID, current.ID); err != nil {
		writeError(w, http.StatusNotFound, "campaign_not_found", "团本不存在")
		return
	}
	items, err := a.diceRolls.ListVisible(r.Context(), current.ID, &campaignID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取团本投骰失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) attachCampaignCharacter(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CharacterID string `json:"characterId"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := a.campaigns.AttachCharacter(r.Context(), r.PathValue("id"), request.CharacterID, currentAccount(r.Context()).ID)
	if !writeCampaignMutationError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) setCampaignCharacterVisibility(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Visibility string `json:"visibility"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := a.campaigns.SetCharacterVisibility(r.Context(), r.PathValue("id"), r.PathValue("characterID"), currentAccount(r.Context()).ID, request.Visibility)
	if !writeCampaignMutationError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) detachCampaignCharacter(w http.ResponseWriter, r *http.Request) {
	err := a.campaigns.DetachCharacter(r.Context(), r.PathValue("id"), r.PathValue("characterID"), currentAccount(r.Context()).ID)
	if !writeCampaignMutationError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
