// ServiceManagement — 服务状态监控、重启、日志
import { useEffect, useState, useCallback } from "react";
import { type ServiceStatus } from "../gen/alfq/v1/broker_pb";
import { serviceManagementClient as client } from "../api/client";

export default function ServiceManagement() {
  const [services, setServices] = useState<ServiceStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [restarting, setRestarting] = useState<string | null>(null);
  const [logs, setLogs] = useState<{ name: string; lines: string[] } | null>(null);
  const [logLoading, setLogLoading] = useState(false);
  const [msg, setMsg] = useState("");

  const load = useCallback(async () => {
    try {
      const res = await client.getServiceStatus({});
      setServices(res.services ?? []);
    } catch (e: unknown) {
      setMsg(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleRestart = async (name: string) => {
    setRestarting(name);
    setMsg("");
    try {
      await client.restartService({ name });
      setMsg(`「${name}」重启指令已发送`);
      setTimeout(load, 3000); // Re-check after restart
    } catch (e: unknown) {
      setMsg(e instanceof Error ? e.message : "重启失败");
    } finally {
      setRestarting(null);
    }
  };

  const handleLogs = async (name: string) => {
    setLogLoading(true);
    try {
      const res = await client.getServiceLogs({ name, tail: 80 });
      setLogs({ name, lines: res.lines ?? [] });
    } catch (e: unknown) {
      setMsg(e instanceof Error ? e.message : "获取日志失败");
    } finally {
      setLogLoading(false);
    }
  };

  const statusColor = (s: string) => {
    if (s === "up") return "var(--color-success)";
    if (s === "down") return "var(--color-danger)";
    return "var(--color-primary)";
  };

  const statusLabel = (s: string) => {
    if (s === "up") return "运行中";
    if (s === "down") return "已停止";
    return "异常";
  };

  return (
    <div className="page">
      <h1 className="page-title">服务管理</h1>
      {msg && <div style={{ color: "var(--color-info)", marginBottom: 12, fontSize: 14 }}>{msg}</div>}

      <div style={{ display: "flex", gap: "1rem", flexWrap: "wrap" }}>
        {/* Service status panel */}
        <div style={{ flex: "1 1 400px" }}>
          <div className="glass-card" style={{ overflow: "auto" }}>
            <div style={{ padding: "1rem 1.5rem", borderBottom: "1px solid var(--color-border)", fontWeight: 600, display: "flex", justifyContent: "space-between" }}>
              <span>服务状态</span>
              <button className="btn-secondary" onClick={load} style={{ padding: "2px 12px", fontSize: 12 }}>刷新</button>
            </div>
            {loading ? (
              <p style={{ padding: "1rem", color: "var(--color-text-muted)" }}>加载中...</p>
            ) : (
              <table className="table">
                <thead><tr><th>服务</th><th>状态</th><th>延迟</th><th>操作</th></tr></thead>
                <tbody>
                  {services.map((s) => (
                    <tr key={s.name}>
                      <td style={{ fontWeight: 600 }}>{s.name}</td>
                      <td><span style={{ color: statusColor(s.status), fontWeight: 600, fontSize: 13 }}>{statusLabel(s.status)}</span></td>
                      <td style={{ color: "var(--color-text-muted)", fontSize: 13 }}>{s.latencyMs > 0 ? `${s.latencyMs}ms` : "—"}</td>
                      <td>
                        <div style={{ display: "flex", gap: 4 }}>
                          <button className="btn-secondary" onClick={() => handleLogs(s.name)} style={{ padding: "2px 10px", fontSize: 12 }}>
                            日志
                          </button>
                          <button className="btn-primary" onClick={() => handleRestart(s.name)}
                            disabled={restarting === s.name}
                            style={{ padding: "2px 10px", fontSize: 12, opacity: restarting === s.name ? 0.6 : 1 }}>
                            {restarting === s.name ? "重启中" : "重启"}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>

        {/* Log viewer */}
        <div style={{ flex: "1 1 500px" }}>
          <div className="glass-card" style={{ height: 500, display: "flex", flexDirection: "column" }}>
            <div style={{ padding: "1rem 1.5rem", borderBottom: "1px solid var(--color-border)", fontWeight: 600, display: "flex", justifyContent: "space-between" }}>
              <span>{logs ? `📋 ${logs.name} 日志` : "运行日志"}</span>
              {logs && (
                <button className="btn-secondary" onClick={() => setLogs(null)} style={{ padding: "2px 12px", fontSize: 12 }}>关闭</button>
              )}
            </div>
            <div style={{ flex: 1, overflow: "auto", padding: "0.75rem", fontFamily: "monospace", fontSize: 12, lineHeight: 1.6, background: "var(--color-bg-secondary)" }} className="scrollbar-thin">
              {logLoading ? (
                <p style={{ color: "var(--color-text-muted)" }}>加载中...</p>
              ) : logs ? (
                logs.lines.map((line, i) => (
                  <div key={i} style={{ color: "var(--color-text-secondary)", whiteSpace: "pre-wrap", wordBreak: "break-all" }}>
                    {line}
                  </div>
                ))
              ) : (
                <p style={{ color: "var(--color-text-muted)" }}>点击左侧「日志」按钮查看服务日志</p>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
