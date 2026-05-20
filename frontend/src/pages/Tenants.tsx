// Tenants page — ALFQ
import { useState, useEffect } from "react";
import PageHeader from "../components/PageHeader";
import DataTable from "../components/DataTable";
import { tenantClient } from "../api/client";
import type { Tenant } from "../gen/alfq/v1/auth_pb";

export default function Tenants() {
  const [items, setItems] = useState<Tenant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    tenantClient.listTenants({})
      .then(res => { setItems(res.tenants ?? []); setLoading(false); })
      .catch((e: unknown) => { setError(e instanceof Error ? e.message : "加载失败"); setLoading(false); });
  }, []);

  return (
    <div style={{ padding: "2rem" }}>
      <PageHeader title="租户管理" description="多租户创建与管理" />
      {error && <div style={{ color: "red", marginTop: 12 }}>{error}</div>}
      {loading ? <p>加载中...</p> : (
        <DataTable
          columns={["ID", "名称", "套餐"]}
          rows={items.map((t: Tenant) => [t.id, t.name, t.plan])}
          emptyText="暂无租户"
        />
      )}
    </div>
  );
}
