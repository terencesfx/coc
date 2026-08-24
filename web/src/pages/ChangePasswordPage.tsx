import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { APIError, api } from "../api/client";
import { useAuth } from "../auth/AuthContext";

export function ChangePasswordPage({
  required = false,
}: {
  required?: boolean;
}) {
  const { logout } = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const newPassword = String(form.get("newPassword"));
    if (newPassword !== String(form.get("confirmPassword"))) {
      setError("两次输入的新密码不一致");
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      await api<void>("/auth/password", {
        method: "PUT",
        body: JSON.stringify({
          currentPassword: form.get("currentPassword"),
          newPassword,
        }),
      });
      await logout();
      navigate("/login", { replace: true });
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "修改密码失败");
    } finally {
      setSubmitting(false);
    }
  }
  return (
    <section className={required ? "auth-page" : ""}>
      <form className="auth-card" onSubmit={(event) => void submit(event)}>
        <p className="eyebrow">{required ? "首次登录" : "账号安全"}</p>
        <h1>修改密码</h1>
        {required && <p>使用其他功能前，请先修改管理员为你设置的初始密码。</p>}
        <label>
          当前密码
          <input
            name="currentPassword"
            type="password"
            autoComplete="current-password"
            required
          />
        </label>
        <label>
          新密码
          <input
            name="newPassword"
            type="password"
            autoComplete="new-password"
            required
          />
        </label>
        <label>
          再次输入新密码
          <input
            name="confirmPassword"
            type="password"
            autoComplete="new-password"
            required
          />
        </label>
        {error && <p className="form-error">{error}</p>}
        <button disabled={submitting}>
          {submitting ? "正在保存…" : "修改密码并重新登录"}
        </button>
      </form>
    </section>
  );
}
