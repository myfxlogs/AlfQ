// NotFound — 404 页面
export default function NotFound() {
  return (
    <div className="page-center" role="alert">
      <h1 style={{ fontSize: "4rem", margin: 0 }}>404</h1>
      <p style={{ color: "var(--text-secondary)", marginTop: 8 }}>页面不存在</p>
      <a href="#/" style={{ marginTop: 16, display: "inline-block" }}>
        返回仪表盘
      </a>
    </div>
  );
}
