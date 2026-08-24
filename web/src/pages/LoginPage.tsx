import { useState, type FormEvent } from "react";
import { APIError } from "../api/client";
import { useAuth } from "../auth/AuthContext";

export function LoginPage() {
  const { login } = useAuth();
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setSubmitting(true);
    setError("");
    try {
      await login(String(form.get("username")), String(form.get("password")));
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "暂时无法登录");
    } finally {
      setSubmitting(false);
    }
  }
  return (
    <main className="auth-page">
      <form className="auth-card" onSubmit={(event) => void submit(event)}>
        <p className="eyebrow">仅限受邀朋友</p>
        <h1>登录 COC7版人物卡</h1>
        <label>
          用户名
          <input name="username" autoComplete="username" required autoFocus />
        </label>
        <label>
          密码
          <input
            name="password"
            type="password"
            autoComplete="current-password"
            required
          />
        </label>
        {error && <p className="form-error">{error}</p>}
        <button disabled={submitting}>
          {submitting ? "正在登录…" : "登录"}
        </button>
      </form>
    </main>
  );
}
