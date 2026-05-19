// AdminSettings — 系统配置（MT4/MT5 网关地址等）
import { useEffect, useState } from "react";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { SystemSettingsService, type SystemSetting } from "../gen/alfq/v1/broker_pb";

const transport = createConnectTransport({ baseUrl: "/api" });
const settingsClient = createClient(SystemSettingsService, transport);

export default function AdminSettings() {
  const [settings, setSettings] = useState<SystemSetting[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState("");

  const load = async () => {
    setLoading(true);
    try {
      const res = await settingsClient.getSystemSettings({});
      setSettings(res.settings ?? []);
    } catch (e: unknown) {
      setMsg(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleEdit = (key: string) => {
    const current = settings.find((s) => s.key === key);
    setEditing((prev) => ({ ...prev, [key]: current?.value ?? "" }));
  };

  const handleSave = async (key: string) => {
    setSaving(true);
    try {
      await settingsClient.updateSystemSetting({ key, value: editing[key] ?? "" });
      setMsg(`「${key}」已保存`);
      setEditing((prev) => { const n = { ...prev }; delete n[key]; return n; });
      load();
    } catch (e: unknown) {
      setMsg(e instanceof Error ? e.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="page">
      <h1 className="page-title">系统配置</h1>
      {msg && <div style={{ color: "var(--color-info)", marginBottom: 12, fontSize: 14 }}>{msg}</div>}

      {loading ? (
        <p style={{ color: "var(--color-text-muted)" }}>加载中...</p>
      ) : (
        <div className="glass-card" style={{ overflow: "auto" }}>
          <table className="table">
            <thead>
              <tr>
                <th>配置项</th>
                <th>值</th>
                <th>说明</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {settings.map((s) => {
                const isEditing = s.key in editing;
                return (
                  <tr key={s.key}>
                    <td style={{ fontWeight: 600, fontFamily: "monospace", fontSize: 13 }}>{s.key}</td>
                    <td>
                      {isEditing ? (
                        <input
                          className="input"
                          value={editing[s.key]}
                          onChange={(e) => setEditing((prev) => ({ ...prev, [s.key]: e.target.value }))}
                          style={{ width: 280 }}
                        />
                      ) : (
                        <span>{s.value}</span>
                      )}
                    </td>
                    <td style={{ color: "var(--color-text-muted)", fontSize: 13 }}>{s.description}</td>
                    <td>
                      {isEditing ? (
                        <div style={{ display: "flex", gap: 4 }}>
                          <button className="btn-primary" onClick={() => handleSave(s.key)} disabled={saving} style={{ padding: "4px 12px", fontSize: 12 }}>
                            保存
                          </button>
                          <button className="btn-secondary" onClick={() => setEditing((prev) => { const n = { ...prev }; delete n[s.key]; return n; })} style={{ padding: "4px 12px", fontSize: 12 }}>
                            取消
                          </button>
                        </div>
                      ) : (
                        <button className="btn-secondary" onClick={() => handleEdit(s.key)} style={{ padding: "4px 12px", fontSize: 12 }}>
                          编辑
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
              {settings.length === 0 && (
                <tr>
                  <td colSpan={4} style={{ textAlign: "center", color: "var(--color-text-muted)", padding: "2rem" }}>
                    暂无配置
                  </td>
                </tr>
              )}
            </tbody>
          </table>
          <div style={{ padding: "1rem", borderTop: "1px solid var(--color-border)", fontSize: 13, color: "var(--color-text-muted)" }}>
            修改网关地址后，需重启 trading-core 服务生效
          </div>
        </div>
      )}
    </div>
  );
}
