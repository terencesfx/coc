import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import {
  APIError,
  api,
  type CheckResult,
  type DiceRoll,
  type ExpressionResult,
  type Campaign,
} from "../api/client";
import { useAuth } from "../auth/AuthContext";

type StoredRoll = DiceRoll<CheckResult | ExpressionResult>;

export function RollsPage() {
  const { account } = useAuth();
  const [items, setItems] = useState<StoredRoll[]>([]);
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [searchParams, setSearchParams] = useSearchParams();
  const campaignID = searchParams.get("campaignId") ?? "";
  const [error, setError] = useState("");
  const [rerolling, setRerolling] = useState("");

  async function reroll(original: StoredRoll) {
    setRerolling(original.id);
    try {
      const item = await api<StoredRoll>("/rolls/reroll", {
        method: "POST",
        body: JSON.stringify({
          requestId: crypto.randomUUID(),
          originalRollId: original.id,
        }),
      });
      setItems((current) => [item, ...current]);
      setError("");
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "重投失败");
    } finally {
      setRerolling("");
    }
  }

  useEffect(() => {
    Promise.all([
      api<{ items: StoredRoll[] }>(
        `/rolls${campaignID ? `?campaignId=${encodeURIComponent(campaignID)}` : ""}`,
      ),
      api<{ items: Campaign[] }>("/campaigns"),
    ])
      .then(([result, campaignResult]) => {
        setItems(result.items ?? []);
        setCampaigns(campaignResult.items ?? []);
      })
      .catch((reason: unknown) =>
        setError(
          reason instanceof APIError ? reason.message : "读取投骰记录失败",
        ),
      );
  }, [campaignID]);

  return (
    <div>
      <div className="page-heading">
        <div>
          <p className="eyebrow">DICE LOG</p>
          <h1>投骰记录</h1>
          <p>公开骰所有人可见；暗骰和测试骰只有投骰者本人可见。</p>
        </div>
      </div>
      {error && <p className="error-banner">{error}</p>}
      <section className="panel">
        <div className="roll-filters">
          <label>
            团本筛选
            <select
              value={campaignID}
              onChange={(event) => {
                const value = event.target.value;
                setSearchParams(value ? { campaignId: value } : {});
              }}
            >
              <option value="">全部记录</option>
              {campaigns.map((item) => (
                <option value={item.id} key={item.id}>
                  {item.title}
                </option>
              ))}
            </select>
          </label>
        </div>
        {items.length === 0 ? (
          <p className="muted">还没有投骰记录。</p>
        ) : (
          <div className="roll-log">
            {items.map((roll) => (
              <article className="roll-log-item" key={roll.id}>
                <div>
                  <strong>{roll.actorName ?? "未知用户"}</strong>
                  <span> · {roll.label}</span>
                  {(roll.rerollKind === "push" ||
                    (!roll.rerollKind &&
                      roll.label.startsWith("孤注一掷"))) && (
                    <small>孤注一掷</small>
                  )}
                  {roll.rerollKind === "reroll" && <small>普通重投</small>}
                  {roll.characterName && <small>{roll.characterName}</small>}
                  {roll.campaignTitle && (
                    <small>团本：{roll.campaignTitle}</small>
                  )}
                </div>
                <RollValue roll={roll} />
                <div className="roll-log-meta">
                  <span>{visibilityName[roll.visibility]}</span>
                  <time>{new Date(roll.createdAt).toLocaleString()}</time>
                  {roll.actorAccountId === account?.id && (
                    <button
                      className="text-button"
                      type="button"
                      disabled={rerolling !== ""}
                      onClick={() => void reroll(roll)}
                    >
                      {rerolling === roll.id ? "重投中…" : "再次投掷"}
                    </button>
                  )}
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function RollValue({ roll }: { roll: StoredRoll }) {
  if (roll.kind === "check") {
    const result = roll.result as CheckResult;
    return (
      <div className={`roll-log-value outcome-${result.outcome}`}>
        <strong>{result.value}</strong>
        <span>
          / {result.target} · {outcomeNames[result.outcome]}
        </span>
        {result.candidates.length > 1 && (
          <small>候选：{result.candidates.join("、")}</small>
        )}
      </div>
    );
  }
  const result = roll.result as ExpressionResult;
  return (
    <div className="roll-log-value">
      <strong>{result.total}</strong>
      <span>{roll.expression}</span>
      <small>
        {result.terms
          .map((term) =>
            term.rolls?.length
              ? `${term.expression}[${term.rolls.join(",")}]`
              : String(term.value),
          )
          .join(" · ")}
      </small>
    </div>
  );
}

const outcomeNames: Record<CheckResult["outcome"], string> = {
  critical: "大成功",
  extreme: "极难成功",
  hard: "困难成功",
  regular: "成功",
  failure: "失败",
  fumble: "大失败",
};

const visibilityName = { public: "公开", keeper: "暗骰", test: "测试" };
