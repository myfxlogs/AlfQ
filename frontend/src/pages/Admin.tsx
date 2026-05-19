// Admin page — ALFQ
import PageHeader from "../components/PageHeader";

export default function Admin() {
  return (
    <div style={{ padding: "2rem" }}>
      <PageHeader title="系统管理" description="用户、租户、权限、系统配置" />
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 16, marginTop: 24 }}>
        <AdminCard title="用户管理" desc="创建、编辑、删除用户，分配角色" href="/users" />
        <AdminCard title="租户管理" desc="管理租户与套餐计划" href="/tenants" />
        <AdminCard title="审计日志" desc="查看操作审计记录" href="/audit" />
        <AdminCard title="系统设置" desc="全局配置与参数" href="/settings" />
      </div>
    </div>
  );
}

function AdminCard({ title, desc, href }: { title: string; desc: string; href: string }) {
  return (
    <a href={href} style={{ border: "1px solid #e8e8e8", borderRadius: 8, padding: 20, textDecoration: "none", color: "inherit", background: "#fafafa" }}>
      <h3 style={{ margin: 0 }}>{title}</h3>
      <p style={{ margin: "8px 0 0", color: "#666", fontSize: 14 }}>{desc}</p>
    </a>
  );
}
