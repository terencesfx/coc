import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { APIError, api, type Campaign } from "../api/client";

export function CampaignsPage() {
  const [items, setItems] = useState<Campaign[]>([]);
  const [title, setTitle] = useState("");
  const [summary, setSummary] = useState("");
  const [error, setError] = useState("");
  const navigate = useNavigate();

  useEffect(() => {
    api<{ items: Campaign[] }>("/campaigns")
      .then((data) => setItems(data.items ?? []))
      .catch(() => setError("读取团本失败"));
  }, []);

  async function create(event: FormEvent) {
    event.preventDefault();
    try {
      const item = await api<Campaign>("/campaigns", {
        method: "POST",
        body: JSON.stringify({ title, summary, status: "preparing" }),
      });
      navigate(`/campaigns/${item.id}`);
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "创建团本失败");
    }
  }

  return (
    <div>
      <div className="page-heading">
        <div>
          <p className="eyebrow">CAMPAIGNS</p>
          <h1>团本</h1>
          <p>所有账号都能查看团本概要，创建者自动成为该团 KP。</p>
        </div>
      </div>
      {error && <p className="error-banner">{error}</p>}
      <section className="panel">
        <h2>创建团本</h2>
        <form
          className="campaign-create"
          onSubmit={(event) => void create(event)}
        >
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            placeholder="团本名称"
            maxLength={120}
            required
          />
          <input
            value={summary}
            onChange={(event) => setSummary(event.target.value)}
            placeholder="一句话简介（可选）"
            maxLength={1000}
          />
          <button className="primary-button" type="submit">
            创建
          </button>
        </form>
      </section>
      <section className="campaign-grid">
        {items.map((item) => (
          <Link
            className="campaign-card"
            to={`/campaigns/${item.id}`}
            key={item.id}
          >
            <div>
              <span className="status-pill">{statusNames[item.status]}</span>
              {item.canManage && <span className="kp-pill">我是 KP</span>}
            </div>
            <h2>{item.title}</h2>
            <p>{item.summary || "暂无简介"}</p>
            <small>
              KP：{item.keeperName} ·{" "}
              {new Date(item.updatedAt).toLocaleDateString()}
            </small>
          </Link>
        ))}
        {items.length === 0 && (
          <p className="muted">还没有团本，可以从上方创建。</p>
        )}
      </section>
    </div>
  );
}

const statusNames: Record<Campaign["status"], string> = {
  preparing: "准备中",
  active: "进行中",
  finished: "已结束",
  archived: "已归档",
};
