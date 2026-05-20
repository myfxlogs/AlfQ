// Users page — ALFQ
import { useState, useEffect } from "react";
import PageHeader from "../components/PageHeader";
import DataTable from "../components/DataTable";
import { userClient } from "../api/client";
import type { User } from "../gen/alfq/v1/auth_pb";

export default function Users() {
  const [items, setItems] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    userClient.listUsers({ tenantId: "" })
      .then(res => { setItems(res.users ?? []); setLoading(false); })
      .catch((e: unknown) => { setError(e instanceof Error ? e.message : "加载失败"); setLoading(false); });
  }, []);

  return (
    <div style={{ padding: "2rem" }}>
      <PageHeader title="用户管理" description="创建、编辑、删除用户与角色分配" />
      {error && <div style={{ color: "red", marginTop: 12 }}>{error}</div>}
      {loading ? <p>加载中...</p> : (
        <DataTable
          columns={["ID", "邮箱", "租户", "角色"]}
          rows={items.map((u: User) => [u.id, u.email, u.tenantId, (u.roles ?? []).join(", ")])}
          emptyText="暂无用户"
        />
      )}
    </div>
  );
}
