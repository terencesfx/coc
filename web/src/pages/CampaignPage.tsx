import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { APIError, api, type Campaign } from "../api/client";
import { CampaignBlocks } from "./CampaignBlocks";
import { CampaignCharacters } from "./CampaignCharacters";
import { CampaignNotifications } from "./CampaignNotifications";
import { CampaignRolls } from "./CampaignRolls";

export function CampaignPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [item, setItem] = useState<Campaign | null>(null);
  const [title, setTitle] = useState("");
  const [summary, setSummary] = useState("");
  const [status, setStatus] = useState<Campaign["status"]>("preparing");
  const [error, setError] = useState("");
  const [playerPreview, setPlayerPreview] = useState(false);
  const [coverUploading, setCoverUploading] = useState(false);

  useEffect(() => {
    api<Campaign>(`/campaigns/${id}`)
      .then((value) => {
        setItem(value);
        setTitle(value.title);
        setSummary(value.summary);
        setStatus(value.status);
      })
      .catch(() => setError("读取团本失败"));
  }, [id]);

  async function save(event?: FormEvent) {
    event?.preventDefault();
    try {
      const updated = await api<Campaign>(`/campaigns/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ title, summary, status }),
      });
      setItem(updated);
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "保存团本失败");
    }
  }

  async function deleteCampaign() {
    if (!item?.canManage || !window.confirm(`确定删除团本“${item.title}”吗？`)) return;
    try {
      await api(`/campaigns/${id}`, { method: "DELETE" });
      navigate("/campaigns");
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "删除团本失败");
    }
  }

  async function uploadCover(file: File) {
    setCoverUploading(true);
    try {
      const form = new FormData();
      form.append("file", file);
      const response = await fetch(`/api/v1/campaigns/${id}/assets`, {
        method: "POST",
        credentials: "same-origin",
        body: form,
      });
      if (!response.ok) {
        const body = (await response.json().catch(() => null)) as {
          message?: string;
        } | null;
        throw new Error(body?.message ?? "上传封面失败");
      }
      const asset = (await response.json()) as { id: string };
      setItem(
        await api<Campaign>(`/campaigns/${id}/cover`, {
          method: "PATCH",
          body: JSON.stringify({ assetId: asset.id }),
        }),
      );
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "上传封面失败");
    } finally {
      setCoverUploading(false);
    }
  }
  async function removeCover() {
    try {
      setItem(
        await api<Campaign>(`/campaigns/${id}/cover`, {
          method: "PATCH",
          body: JSON.stringify({ assetId: null }),
        }),
      );
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "移除封面失败");
    }
  }

  if (!item)
    return <div className="center-message">{error || "正在读取团本……"}</div>;
  return (
    <div className="campaign-page-stack">
      <header className="campaign-page-header">
        <Link className="back-link" to="/campaigns">← 返回团本列表</Link>
        {item.canManage && !playerPreview && <div className="page-save-actions">
          <button type="button" onClick={() => void save()}>保存团本</button>
          <button className="danger-action" type="button" onClick={() => void deleteCampaign()}>删除团本</button>
        </div>}
      </header>
      <div className="campaign-hero">
        {item.coverAssetId && (
          <img
            className="campaign-cover"
            src={`/api/v1/campaigns/${id}/assets/${item.coverAssetId}`}
            alt={`${item.title}封面`}
          />
        )}
        <p className="eyebrow">
          {item.canManage && !playerPreview ? "KEEPER VIEW" : "PLAYER VIEW"}
        </p>
        <h1>{item.title}</h1>
        <p>{item.summary || "暂无简介"}</p>
        <small>KP：{item.keeperName}</small>
        {item.canManage && (
          <div className="campaign-view-actions">
            <button
              type="button"
              onClick={() => setPlayerPreview((value) => !value)}
            >
              {playerPreview ? "返回 KP 视角" : "预览玩家视角"}
            </button>
            {!playerPreview && (
              <label className="button-link">
                {coverUploading ? "上传中……" : "上传封面"}
                <input
                  hidden
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  disabled={coverUploading}
                  onChange={(event) => {
                    const file = event.target.files?.[0];
                    if (file) void uploadCover(file);
                  }}
                />
              </label>
            )}
            {!playerPreview && item.coverAssetId && (
              <button type="button" onClick={() => void removeCover()}>
                移除封面
              </button>
            )}
          </div>
        )}
      </div>
      {error && <p className="error-banner">{error}</p>}
      {item.canManage && !playerPreview && (
        <section className="panel">
          <h2>团本设置</h2>
          <form
            className="campaign-settings"
            onSubmit={(event) => void save(event)}
          >
            <label>
              团本名称
              <input
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                maxLength={120}
                required
              />
            </label>
            <label>
              状态
              <select
                value={status}
                onChange={(event) =>
                  setStatus(event.target.value as Campaign["status"])
                }
              >
                <option value="preparing">准备中</option>
                <option value="active">进行中</option>
                <option value="finished">已结束</option>
                <option value="archived">已归档</option>
              </select>
            </label>
            <label className="full-row">
              简介
              <textarea
                rows={4}
                value={summary}
                onChange={(event) => setSummary(event.target.value)}
                maxLength={1000}
              />
            </label>
          </form>
        </section>
      )}
      <CampaignBlocks
        campaignID={id}
        canManage={item.canManage}
        playerPreview={playerPreview}
      />
      <CampaignCharacters
        campaignID={id}
        canManage={item.canManage}
        playerPreview={playerPreview}
      />
      <CampaignRolls campaignID={id} />
      {item.canManage && !playerPreview && (
        <CampaignNotifications campaignID={id} />
      )}
    </div>
  );
}
