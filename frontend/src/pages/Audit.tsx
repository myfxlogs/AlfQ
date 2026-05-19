// Audit page — ALFQ
import { useState, useEffect } from "react";
import PageHeader from "../components/PageHeader";
import DataTable from "../components/DataTable";
import { apiFetch } from "../api/client";
import type { AuditLog, ListAuditLogsResponse } from "../gen/alfq/v1/auth_pb";

export default function Audit() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    apiFetch<ListAuditLogsResponse>("/alfq.v1.AuditService/ListAuditLogs", {
      method: "POST",
      body: JSON.stringify({ tenantId: "", limit: 50 }),
    })
      .then(res => { setLogs(res.logs ?? []); setLoading(false); })
      .catch((e: unknown) => { setError(e instanceof Error ? e.message : "加载失败"); setLoading(false); });
  }, []);

  return (
    <div style={{ padding: "2rem" }}>
      <PageHeader title="审计日志" description="操作审计追踪记录" />
      {error && <div style={{ color: "red", marginTop: 12 }}>{error}</div>}
      {loading ? <p>加载中...</p> : (
        <DataTable
          columns={["ID", "用户", "操作", "资源", "时间"]}
          rows={logs.map((l: AuditLog) => [l.id, l.userId, l.action, l.resource, new Date(Number(l.tsUnixMs)).toLocaleString()])}
          emptyText="暂无审计日志"
        />
      )}
    </div>
  );
}
