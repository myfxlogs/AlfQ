// Unauthorized — 未授权/会话过期提示页
import { clearToken } from "../api/client";

interface Props {
  reason?: string;
}

export default function Unauthorized({ reason }: Props) {
  const msg =
    reason === "expired"
      ? "会话已过期，请重新登录"
      : reason === "forbidden"
        ? "权限不足，无法访问此页面"
        : "未授权，请先登录";

  return (
    <div className="page-center" role="alert">
      <h1 style={{ fontSize: "2rem", margin: 0 }}>🔒 未授权</h1>
      <p style={{ color: "var(--text-secondary)", marginTop: 8 }}>{msg}</p>
      <a
        href={`#/login?reason=${reason || "unauthorized"}`}
        style={{ marginTop: 16, display: "inline-block" }}
        onClick={() => clearToken()}
      >
        前往登录
      </a>
    </div>
  );
}
