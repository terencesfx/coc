import { useEffect, useState } from "react";
import { APIError, api } from "../api/client";

type SystemStatus = {
  startedAt: string;
  uptimeSeconds: number;
  databaseBytes: number;
  accountCount: number;
  activeSessions: number;
  backupCount: number;
  latestBackupAt: string | null;
};
type Backup = { name: string; sizeBytes: number; createdAt: string };
type ValidationReport = {
  valid: boolean;
  databaseIntegrity: string;
  assetCount: number;
  hasCustomOccupations: boolean;
  fileCount: number;
};
type AuditLog = {
  id: string;
  actorDisplayName: string;
  action: string;
  createdAt: string;
};

export function SystemPage() {
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const [validationFile, setValidationFile] = useState<File | null>(null);
  const [validation, setValidation] = useState<ValidationReport | null>(null);
  const [validating, setValidating] = useState(false);

  async function load() {
    const [nextStatus, backupResult, logResult] = await Promise.all([
      api<SystemStatus>("/admin/system/status"),
      api<{ items: Backup[] }>("/admin/backups"),
      api<{ items: AuditLog[] }>("/admin/audit-logs"),
    ]);
    setStatus(nextStatus);
    setBackups(backupResult.items ?? []);
    setLogs(logResult.items ?? []);
  }

  async function validateBackup() {
    if (!validationFile) return;
    setValidating(true);
    setError("");
    setValidation(null);
    try {
      const form = new FormData();
      form.append("file", validationFile);
      const response = await fetch("/api/v1/admin/backups/validate", {
        method: "POST",
        credentials: "same-origin",
        body: form,
      });
      if (!response.ok) {
        const body = (await response.json().catch(() => null)) as {
          message?: string;
        } | null;
        throw new Error(body?.message ?? "备份校验失败");
      }
      setValidation((await response.json()) as ValidationReport);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "备份校验失败");
    } finally {
      setValidating(false);
    }
  }

  useEffect(() => {
    let active = true;
    Promise.all([
      api<SystemStatus>("/admin/system/status"),
      api<{ items: Backup[] }>("/admin/backups"),
      api<{ items: AuditLog[] }>("/admin/audit-logs"),
    ])
      .then(([nextStatus, backupResult, logResult]) => {
        if (!active) return;
        setStatus(nextStatus);
        setBackups(backupResult.items ?? []);
        setLogs(logResult.items ?? []);
      })
      .catch((reason: unknown) => {
        if (active)
          setError(
            reason instanceof APIError ? reason.message : "读取系统信息失败",
          );
      });
    return () => {
      active = false;
    };
  }, []);

  async function createBackup() {
    setCreating(true);
    setError("");
    try {
      await api<Backup>("/admin/backups", { method: "POST" });
      await load();
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "创建备份失败");
    } finally {
      setCreating(false);
    }
  }

  return (
    <section className="page-stack">
      <header className="page-header">
        <div>
          <p className="eyebrow">系统管理</p>
          <h1>维护与备份</h1>
        </div>
        <button
          type="button"
          disabled={creating}
          onClick={() => void createBackup()}
        >
          {creating ? "正在备份…" : "创建完整备份"}
        </button>
      </header>
      {error && <p className="form-error">{error}</p>}
      {status && (
        <div className="stat-grid">
          <Stat label="账号" value={String(status.accountCount)} />
          <Stat label="有效 Session" value={String(status.activeSessions)} />
          <Stat label="数据库" value={formatBytes(status.databaseBytes)} />
          <Stat label="运行时间" value={formatDuration(status.uptimeSeconds)} />
        </div>
      )}
      <section className="panel">
        <h2>完整数据备份</h2>
        <p className="muted">
          备份包包含 SQLite 数据库、团本图片和自定义职业 JSON。
        </p>
        <div className="simple-list">
          {backups.length === 0 && <p className="muted">尚未创建备份。</p>}
          {backups.map((item) => (
            <div className="simple-row" key={item.name}>
              <span>
                <strong>{new Date(item.createdAt).toLocaleString()}</strong>
                <small>
                  {formatBytes(item.sizeBytes)} · {item.name}
                </small>
              </span>
              <a
                className="button-link"
                href={`/api/v1/admin/backups/${encodeURIComponent(item.name)}`}
              >
                下载
              </a>
            </div>
          ))}
        </div>
      </section>
      <section className="panel">
        <h2>校验恢复包</h2>
        <p className="muted">
          这里只读取并校验备份，不会覆盖当前数据。正式恢复需要停服确认。
        </p>
        <div className="backup-validation">
          <input
            type="file"
            accept=".gz,application/gzip"
            onChange={(event) =>
              setValidationFile(event.target.files?.[0] ?? null)
            }
          />
          <button
            type="button"
            disabled={!validationFile || validating}
            onClick={() => void validateBackup()}
          >
            {validating ? "校验中……" : "上传并校验"}
          </button>
        </div>
        {validation?.valid && (
          <div className="validation-result">
            <strong>备份包完整</strong>
            <span>数据库：{validation.databaseIntegrity}</span>
            <span>图片：{validation.assetCount} 个</span>
            <span>
              自定义职业：{validation.hasCustomOccupations ? "包含" : "未包含"}
            </span>
            <span>文件总数：{validation.fileCount}</span>
          </div>
        )}
      </section>
      <section className="panel">
        <h2>管理员操作审计</h2>
        <div className="simple-list">
          {logs.length === 0 && <p className="muted">暂无管理员操作。</p>}
          {logs.map((item) => (
            <div className="simple-row" key={item.id}>
              <span>
                <strong>
                  {item.actorDisplayName} ·{" "}
                  {actionLabel[item.action] ?? item.action}
                </strong>
                <small>{new Date(item.createdAt).toLocaleString()}</small>
              </span>
            </div>
          ))}
        </div>
      </section>
    </section>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <article className="stat-card">
      <span>{label}</span>
      <strong>{value}</strong>
    </article>
  );
}
function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}
function formatDuration(seconds: number) {
  if (seconds < 60) return `${seconds} 秒`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟`;
  return `${Math.floor(seconds / 3600)} 小时`;
}
const actionLabel: Record<string, string> = {
  "account.create": "创建账号",
  "account.reset_password": "重置密码",
  "account.set_status": "更改账号状态",
  "account.revoke_sessions": "撤销登录",
  "backup.create": "创建备份",
  "backup.download": "下载备份",
  "backup.validate": "校验恢复包",
};
