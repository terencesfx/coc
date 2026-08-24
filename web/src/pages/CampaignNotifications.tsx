import { useEffect, useState } from "react";
import { APIError, api } from "../api/client";

type Setting = {
  campaignId: string;
  provider: "disabled" | "console";
  targetReference: string;
  updatedAt: string;
};
type Delivery = {
  id: string;
  rollId: string;
  provider: string;
  status: "pending" | "sending" | "sent" | "failed" | "skipped";
  attempts: number;
  lastError: string;
  createdAt: string;
  updatedAt: string;
};

export function CampaignNotifications({ campaignID }: { campaignID: string }) {
  const [provider, setProvider] = useState<Setting["provider"]>("disabled");
  const [target, setTarget] = useState("");
  const [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  async function load() {
    const [setting, result] = await Promise.all([
      api<Setting>(`/campaigns/${campaignID}/notifications`),
      api<{ items: Delivery[] }>(
        `/campaigns/${campaignID}/notification-deliveries`,
      ),
    ]);
    setProvider(setting.provider);
    setTarget(setting.targetReference);
    setDeliveries(result.items ?? []);
  }
  useEffect(() => {
    let active = true;
    Promise.all([
      api<Setting>(`/campaigns/${campaignID}/notifications`),
      api<{ items: Delivery[] }>(
        `/campaigns/${campaignID}/notification-deliveries`,
      ),
    ])
      .then(([setting, result]) => {
        if (active) {
          setProvider(setting.provider);
          setTarget(setting.targetReference);
          setDeliveries(result.items ?? []);
        }
      })
      .catch(() => {
        if (active) setError("读取通知设置失败");
      });
    return () => {
      active = false;
    };
  }, [campaignID]);
  async function save() {
    try {
      await api(`/campaigns/${campaignID}/notifications`, {
        method: "PUT",
        body: JSON.stringify({ provider, targetReference: target }),
      });
      setMessage(
        provider === "console"
          ? "控制台测试通知已启用，之后的公开团本投骰会异步输出到后端日志。"
          : "通知已关闭。",
      );
      setError("");
      await load();
    } catch (reason) {
      setError(
        reason instanceof APIError ? reason.message : "保存通知设置失败",
      );
    }
  }
  return (
    <section className="panel">
      <h2>投骰通知</h2>
      <p className="muted">
        目前用于本地测试异步通知流程。真实 QQ 官方机器人将在部署阶段接入。
      </p>
      {error && <p className="error-banner">{error}</p>}
      {message && <p className="success-banner">{message}</p>}
      <div className="notification-settings">
        <label>
          通知通道
          <select
            value={provider}
            onChange={(event) =>
              setProvider(event.target.value as Setting["provider"])
            }
          >
            <option value="disabled">关闭</option>
            <option value="console">控制台测试</option>
          </select>
        </label>
        <label>
          测试目标标识
          <input
            value={target}
            onChange={(event) => setTarget(event.target.value)}
            placeholder="例如：朋友群测试"
          />
        </label>
        <button
          className="primary-button"
          type="button"
          onClick={() => void save()}
        >
          保存通知设置
        </button>
      </div>
      <h3>最近投递</h3>
      <div className="simple-list">
        {deliveries.map((item) => (
          <div className="simple-row" key={item.id}>
            <span>
              <strong>
                {statusNames[item.status]} · {item.provider}
              </strong>
              <small>
                {new Date(item.createdAt).toLocaleString()} · 尝试{" "}
                {item.attempts} 次{item.lastError ? ` · ${item.lastError}` : ""}
              </small>
            </span>
          </div>
        ))}
        {deliveries.length === 0 && (
          <p className="muted">
            还没有通知任务。启用后，新产生的公开团本投骰会创建任务。
          </p>
        )}
      </div>
    </section>
  );
}
const statusNames: Record<Delivery["status"], string> = {
  pending: "等待发送",
  sending: "发送中",
  sent: "已发送",
  failed: "发送失败",
  skipped: "已跳过",
};
