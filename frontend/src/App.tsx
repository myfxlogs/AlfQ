// ALFQ App — sidebar layout + responsive mobile
import { useState, useEffect, useCallback } from "react";
import { authClient, getToken, clearToken } from "./api/client";
import Dashboard from "./pages/Dashboard";
import Login from "./pages/Login";
import Orders from "./pages/Orders";
import Positions from "./pages/Positions";
import RiskRules from "./pages/RiskRules";
import Strategies from "./pages/Strategies";
import Backtest from "./pages/Backtest";
import AIChat from "./pages/AIChat";
import Audit from "./pages/Audit";
import Notifications from "./pages/Notifications";
import Settings from "./pages/Settings";
import Users from "./pages/Users";
import AdminSettings from "./pages/AdminSettings";
import ServiceManagement from "./pages/ServiceManagement";
import BindAccount from "./pages/BindAccount";

const userRoutes: Record<string, () => React.ReactNode> = {
  "#/": Dashboard,
  "#/bind": () => <BindAccount />,
  "#/orders": Orders,
  "#/positions": Positions,
  "#/risk": RiskRules,
  "#/strategies": Strategies,
  "#/backtest": Backtest,
  "#/assistant": AIChat,
  "#/audit": Audit,
  "#/notifications": Notifications,
  "#/settings": Settings,
};

const adminRoutes: Record<string, () => React.ReactNode> = {
  "#/admin/users": Users,
  "#/admin/settings": AdminSettings,
  "#/admin/services": ServiceManagement,
};

const userNavItems: [string, string][] = [
  ["#/", "仪表盘"],
  ["#/orders", "订单"],
  ["#/positions", "持仓"],
  ["#/risk", "风控规则"],
  ["#/strategies", "策略"],
  ["#/backtest", "回测"],
  ["#/assistant", "AI 助手"],
  ["#/audit", "审计日志"],
  ["#/notifications", "通知"],
];

function useHash(): string {
  const [hash, setHash] = useState(window.location.hash || "#/");
  useEffect(() => {
    const cb = () => setHash(window.location.hash || "#/");
    window.addEventListener("hashchange", cb);
    return () => window.removeEventListener("hashchange", cb);
  }, []);
  return hash;
}

function Sidebar({ hash, isAdmin, onClose }: { hash: string; isAdmin: boolean; onClose: () => void }) {
  const token = getToken();
  const loggedIn = !!token;

  const handleLogout = async () => {
    try {
      if (token) {
        await authClient.logout({ accessToken: token });
      }
    } catch {
      // 即使后端调用失败也清除本地状态
    }
    clearToken();
    localStorage.removeItem("alfq_email");
    window.location.hash = "#/login";
    window.location.reload();
  };

  return (
    <>
      <div className={`sidebar${isAdmin ? "" : ""}`} id="sidebar">
        {isAdmin ? (
          <>
            <div className="sidebar-brand">ALFQ 管理</div>
            <div className="sidebar-nav">
              <a href="#/admin/users" className={`sidebar-link${hash === "#/admin/users" ? " active" : ""}`} onClick={onClose}>
                用户管理
              </a>
              <a href="#/admin/settings" className={`sidebar-link${hash === "#/admin/settings" ? " active" : ""}`} onClick={onClose}>
                系统配置
              </a>
              <a href="#/admin/services" className={`sidebar-link${hash === "#/admin/services" ? " active" : ""}`} onClick={onClose}>
                服务管理
              </a>
            </div>
            <div className="sidebar-footer">
              <a href="#/" className="sidebar-footer-link">← 返回交易端</a>
            </div>
          </>
        ) : (
          <>
            <div className="sidebar-brand">ALFQ</div>
            <div className="sidebar-nav">
              {userNavItems.map(([path, label]) => (
                <a key={path} href={path} className={`sidebar-link${hash === path ? " active" : ""}`} onClick={onClose}>
                  {label}
                </a>
              ))}
            </div>
            <div className="sidebar-footer">
              <a href="#/settings" className="sidebar-footer-link">设置</a>
              <a href="#/admin/tenants" className="sidebar-footer-link">管理端</a>
              {loggedIn ? (
                <a
                  href="#"
                  className="sidebar-footer-link"
                  onClick={(e) => { e.preventDefault(); handleLogout(); }}
                >
                  退出
                </a>
              ) : (
                <a href="#/login" className="sidebar-footer-link">登录</a>
              )}
            </div>
          </>
        )}
      </div>
      <div className={`sidebar-overlay${isAdmin ? "" : ""}`} id="sidebar-overlay" onClick={onClose} />
    </>
  );
}

export default function App() {
  const hash = useHash();
  const isAdmin = hash.startsWith("#/admin/");
  const [sidebarOpen, setSidebarOpen] = useState(false);

  const closeSidebar = useCallback(() => setSidebarOpen(false), []);
  const toggleSidebar = useCallback(() => setSidebarOpen((v) => !v), []);

  // Sync sidebar state with overlay
  useEffect(() => {
    const sidebar = document.getElementById("sidebar");
    const overlay = document.getElementById("sidebar-overlay");
    if (sidebarOpen) {
      sidebar?.classList.add("open");
      overlay?.classList.add("open");
    } else {
      sidebar?.classList.remove("open");
      overlay?.classList.remove("open");
    }
  }, [sidebarOpen]);

  // Login page is fullscreen, no layout
  if (hash === "#/login") {
    return <Login />;
  }

  const Page = isAdmin ? adminRoutes[hash] || Users : userRoutes[hash] || Dashboard;

  return (
    <div className="app-layout">
      <Sidebar hash={hash} isAdmin={isAdmin} onClose={closeSidebar} />

      <div className="main-content">
        {/* Mobile top bar */}
        <div className="topbar">
          <button className="hamburger" onClick={toggleSidebar} aria-label="菜单">
            ☰
          </button>
          <span className="topbar-brand">{isAdmin ? "ALFQ 管理" : "ALFQ"}</span>
        </div>

        <Page />
      </div>
    </div>
  );
}