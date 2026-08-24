import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  APIError,
  api,
  type CampaignCharacter,
  type CharacterSummary,
} from "../api/client";

export function CampaignCharacters({
  campaignID,
  canManage,
  playerPreview = false,
}: {
  campaignID: string;
  canManage: boolean;
  playerPreview?: boolean;
}) {
  const [items, setItems] = useState<CampaignCharacter[]>([]);
  const [own, setOwn] = useState<CharacterSummary[]>([]);
  const [selected, setSelected] = useState("");
  const [error, setError] = useState("");
  async function load() {
    const result = await api<{ items: CampaignCharacter[] }>(
      `/campaigns/${campaignID}/characters`,
    );
    setItems(result.items ?? []);
  }
  useEffect(() => {
    let active = true;
    Promise.all([
      api<{ items: CampaignCharacter[] }>(
        `/campaigns/${campaignID}/characters`,
      ),
      api<{ items: CharacterSummary[] }>("/characters"),
    ])
      .then(([linked, characters]) => {
        if (active) {
          setItems(linked.items ?? []);
          setOwn(characters.items ?? []);
        }
      })
      .catch(() => {
        if (active) setError("读取团本人物失败");
      });
    return () => {
      active = false;
    };
  }, [campaignID]);
  const available = own.filter(
    (item) =>
      !items.some((linked) => linked.characterId === item.id) &&
      (canManage || item.kind === "investigator"),
  );
  async function attach() {
    if (!selected) return;
    try {
      await api(`/campaigns/${campaignID}/characters`, {
        method: "POST",
        body: JSON.stringify({ characterId: selected }),
      });
      setSelected("");
      await load();
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "挂靠人物失败");
    }
  }
  async function setVisibility(
    characterID: string,
    visibility: "public" | "hidden",
  ) {
    await api(`/campaigns/${campaignID}/characters/${characterID}`, {
      method: "PATCH",
      body: JSON.stringify({ visibility }),
    });
    await load();
  }
  async function detach(characterID: string) {
    if (!window.confirm("确定解除挂靠吗？人物卡本身和历史不会删除。")) return;
    await api(`/campaigns/${campaignID}/characters/${characterID}`, {
      method: "DELETE",
    });
    await load();
  }
  return (
    <section className="panel">
      <h2>登场人物</h2>
      {error && <p className="error-banner">{error}</p>}
      <div className="campaign-character-list">
        {items
          .filter((item) => !playerPreview || item.visibility === "public")
          .map((item) => (
            <div className="campaign-character-row" key={item.characterId}>
              <Link
                to={`/characters/${item.characterId}?campaignId=${campaignID}`}
              >
                <strong>{item.name}</strong>
                <small>
                  {item.role === "npc" ? "NPC" : "调查员"} · {item.ownerName}
                </small>
              </Link>
              <span>{item.visibility === "public" ? "公开" : "仅 KP"}</span>
              {!playerPreview && (
                <div className="row-actions">
                  {canManage && item.role === "npc" && (
                    <button
                      type="button"
                      onClick={() =>
                        void setVisibility(
                          item.characterId,
                          item.visibility === "public" ? "hidden" : "public",
                        )
                      }
                    >
                      {item.visibility === "public" ? "隐藏" : "公开"}
                    </button>
                  )}
                  {(canManage ||
                    own.some((ownItem) => ownItem.id === item.characterId)) && (
                    <button
                      type="button"
                      onClick={() => void detach(item.characterId)}
                    >
                      解除
                    </button>
                  )}
                </div>
              )}
            </div>
          ))}
        {items.length === 0 && (
          <p className="muted">还没有人物卡挂靠到本团。</p>
        )}
      </div>
      {!playerPreview && (
        <div className="attach-character">
          <select
            value={selected}
            onChange={(event) => setSelected(event.target.value)}
          >
            <option value="">
              选择自己的{canManage ? "调查员或 NPC" : "调查员"}
            </option>
            {available.map((item) => (
              <option value={item.id} key={item.id}>
                {item.name}（{item.kind === "npc" ? "NPC" : "调查员"}）
              </option>
            ))}
          </select>
          <button
            className="primary-button"
            type="button"
            disabled={!selected}
            onClick={() => void attach()}
          >
            加入
          </button>
        </div>
      )}
    </section>
  );
}
