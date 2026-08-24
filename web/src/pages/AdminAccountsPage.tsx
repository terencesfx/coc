import { useEffect, useState, type FormEvent } from "react";
import { APIError, api, type Account } from "../api/client";
import { useAuth } from "../auth/AuthContext";

export function AdminAccountsPage() {
  const { account: current } = useAuth();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [error, setError] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  async function load() {
    try {
      const result = await api<{ items: Account[] }>("/admin/accounts");
      setAccounts(result.items ?? []);
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "读取账号失败");
    }
  }
  useEffect(() => {
    let active = true;
    api<{ items: Account[] }>("/admin/accounts")
      .then((result) => {
        if (active) setAccounts(result.items ?? []);
      })
      .catch((reason: unknown) => {
        if (active)
          setError(
            reason instanceof APIError ? reason.message : "读取账号失败",
          );
      });
    return () => {
      active = false;
    };
  }, []);
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      await api<Account>("/admin/accounts", {
        method: "POST",
        body: JSON.stringify({
          username: form.get("username"),
          displayName: form.get("displayName"),
          password: form.get("password"),
        }),
      });
      event.currentTarget.reset();
      setShowCreate(false);
      await load();
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "创建账号失败");
    }
  }
  async function perform(path: string, init: RequestInit) {
    try {
      setError("");
      await api<void>(path, init);
      await load();
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "操作失败");
    }
  }
  async function toggleStatus(item: Account) {
    const status = item.status === "active" ? "disabled" : "active";
    if (
      status === "disabled" &&
      !window.confirm(`确定停用 ${item.displayName} 吗？`)
    )
      return;
    await perform(`/admin/accounts/${item.id}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    });
  }
  async function resetPassword(item: Account) {
    const password = window.prompt(
      `为 ${item.displayName} 设置新的初始密码`,
    );
    if (!password) return;
    await perform(`/admin/accounts/${item.id}/reset-password`, {
      method: "POST",
      body: JSON.stringify({ password }),
    });
  }
  return (
    <section className="page-stack">
      <header className="page-header">
        <div>
          <p className="eyebrow">系统管理</p>
          <h1>朋友账号</h1>
        </div>
        <button type="button" onClick={() => setShowCreate((value) => !value)}>
          {showCreate ? "取消" : "创建账号"}
        </button>
      </header>
      {showCreate && (
        <form className="inline-form" onSubmit={(event) => void create(event)}>
          <label>
            用户名
            <input name="username" required />
          </label>
          <label>
            显示名称
            <input name="displayName" required />
          </label>
          <label>
            初始密码
            <input name="password" type="password" required />
          </label>
          <button>保存</button>
        </form>
      )}
      {error && <p className="form-error">{error}</p>}
      <div className="account-list">
        {accounts.map((item) => (
          <article className="account-row" key={item.id}>
            <div className="account-identity">
              <strong>{item.displayName}</strong>
              <span>
                @{item.username} · {item.role === "admin" ? "管理员" : "用户"}
              </span>
              <small>
                最近登录：
                {item.lastLoginAt
                  ? new Date(item.lastLoginAt).toLocaleString()
                  : "从未登录"}
              </small>
            </div>
            <span className={`status status--${item.status}`}>
              {item.status === "active" ? "有效" : "已停用"}
            </span>
            <div className="row-actions">
              <button
                className="secondary"
                type="button"
                onClick={() => void resetPassword(item)}
              >
                重置密码
              </button>
              <button
                className="secondary"
                type="button"
                onClick={() =>
                  void perform(`/admin/accounts/${item.id}/revoke-sessions`, {
                    method: "POST",
                  })
                }
              >
                撤销登录
              </button>
              <button
                className="secondary"
                type="button"
                disabled={item.id === current?.id}
                onClick={() => void toggleStatus(item)}
              >
                {item.status === "active" ? "停用" : "恢复"}
              </button>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}
