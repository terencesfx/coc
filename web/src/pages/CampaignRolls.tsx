import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  APIError,
  api,
  type CheckResult,
  type DiceRoll,
  type ExpressionResult,
} from "../api/client";
import { useAuth } from "../auth/AuthContext";

type StoredRoll = DiceRoll<CheckResult | ExpressionResult>;

export function CampaignRolls({ campaignID }: { campaignID: string }) {
  const { account } = useAuth();
  const [items, setItems] = useState<StoredRoll[]>([]);
  const [error, setError] = useState("");
  const [refreshing, setRefreshing] = useState(false);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
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
      setItems((current) => [item, ...current].slice(0, 20));
      setError("");
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "重投失败");
    } finally {
      setRerolling("");
    }
  }

  const load = useCallback(
    async (showProgress = false) => {
      if (showProgress) setRefreshing(true);
      try {
        const result = await api<{ items: StoredRoll[] }>(
          `/campaigns/${campaignID}/rolls`,
        );
        setItems((result.items ?? []).slice(0, 20));
        setUpdatedAt(new Date());
        setError("");
      } catch (reason) {
        setError(
          reason instanceof APIError ? reason.message : "读取团本投骰失败",
        );
      } finally {
        if (showProgress) setRefreshing(false);
      }
    },
    [campaignID],
  );

  useEffect(() => {
    const initial = window.setTimeout(() => void load(), 0);
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") void load();
    }, 4000);
    const refreshVisible = () => {
      if (document.visibilityState === "visible") void load();
    };
    document.addEventListener("visibilitychange", refreshVisible);
    return () => {
      window.clearTimeout(initial);
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", refreshVisible);
    };
  }, [load]);

  return (
    <section className="panel">
      <div className="section-heading">
        <div>
          <h2>团本投骰</h2>
          <small className="muted">
            {updatedAt
              ? `最近更新：${updatedAt.toLocaleTimeString()}`
              : "正在读取……"}
          </small>
        </div>
        <div className="row-actions">
          <button
            type="button"
            disabled={refreshing}
            onClick={() => void load(true)}
          >
            {refreshing ? "刷新中…" : "立即刷新"}
          </button>
          <Link className="button-link campaign-roll-history-link" to={`/rolls?campaignId=${campaignID}`}>
            查看完整记录
          </Link>
        </div>
      </div>
      <p className="muted">
        页面可见时每 4 秒自动刷新；暗骰仍只对投骰者和本团 KP 可见。
      </p>
      {error && <p className="error-banner">{error}</p>}
      {items.length === 0 && !error ? (
        <p className="muted">本团还没有投骰记录。</p>
      ) : (
        <div className="campaign-roll-feed" aria-live="polite">
          {items.map((roll) => (
            <article className="campaign-roll-item" key={roll.id}>
              <div>
                <strong>{roll.actorName ?? "未知用户"}</strong>
                {roll.characterName && <small>{roll.characterName}</small>}
              </div>
              <div>
                <span>{roll.label}</span>
                {(roll.rerollKind === "push" ||
                  (!roll.rerollKind && roll.label.startsWith("孤注一掷"))) && (
                  <small>孤注一掷</small>
                )}
                {roll.rerollKind === "reroll" && <small>普通重投</small>}
              </div>
              <CampaignRollValue roll={roll} />
              <div className="roll-log-meta">
                <span>{visibilityNames[roll.visibility]}</span>
                <time>{new Date(roll.createdAt).toLocaleTimeString()}</time>
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
  );
}

function CampaignRollValue({ roll }: { roll: StoredRoll }) {
  if (roll.kind === "check") {
    const result = roll.result as CheckResult;
    return (
      <div className={`roll-log-value outcome-${result.outcome}`}>
        <strong>{result.value}</strong>
        <span>
          / {result.target} · {outcomeNames[result.outcome]}
        </span>
      </div>
    );
  }
  const result = roll.result as ExpressionResult;
  return (
    <div className="roll-log-value">
      <strong>{result.total}</strong>
      <span>{roll.expression}</span>
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

const visibilityNames = { public: "公开", keeper: "暗骰", test: "测试" };
