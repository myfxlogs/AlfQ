// ALFQ App — hash-based routing with user/admin split
import { useState, useEffect } from "react";
import Dashboard from "./pages/Dashboard";
import Login from "./pages/Login";
import Accounts from "./pages/Accounts";
import Orders from "./pages/Orders";
import Positions from "./pages/Positions";
import RiskRules from "./pages/RiskRules";
import Strategies from "./pages/Strategies";
import Backtest from "./pages/Backtest";
import AIChat from "./pages/AIChat";
import Audit from "./pages/Audit";
import Notifications from "./pages/Notifications";
import Settings from "./pages/Settings";
import Tenants from "./pages/Tenants";
import Users from "./pages/Users";

const userRoutes: Record<string, () => React.ReactNode> = {
  "#/": Dashboard,
  "#/accounts": Accounts,
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
  "#/admin/tenants": Tenants,
  "#/admin/users": Users,
};

const userNavItems: [string, string][] = [
  ["#/", "仪表盘"],
  ["#/accounts", "账户"],
  ["#/orders", "订单"],
  ["#/positions", "持仓"],
  ["#/risk", "风控"],
  ["#/strategies", "策略"],
  ["#/backtest", "回测"],
  ["#/assistant", "AI助手"],
  ["#/audit", "审计"],
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

export default function App() {
  const hash = useHash();
  const isAdmin = hash.startsWith("#/admin/");

  // Admin layout
  if (isAdmin) {
    const Page = adminRoutes[hash];
    return (
      <div>
        <nav className="navbar">
          <a href="#/" className="navbar-brand">ALFQ</a>
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14, marginLeft: 8 }}>管理端</span>
          <a href="#/admin/tenants" className={`nav-link${hash === "#/admin/tenants" ? " active" : ""}`}>租户</a>
          <a href="#/admin/users" className={`nav-link${hash === "#/admin/users" ? " active" : ""}`}>用户</a>
          <a href="#/" className="nav-login">返回用户端</a>
        </nav>
        {Page ? <Page /> : <div className="page-placeholder">页面不存在</div>}
      </div>
    );
  }

  // User layout
  const Page = userRoutes[hash] || Dashboard;
  return (
    <div>
      <nav className="navbar">
        <a href="#/" className="navbar-brand">ALFQ</a>
        {userNavItems.map(([path, label]) => (
          <a key={path} href={path} className={`nav-link${hash === path ? " active" : ""}`}>{label}</a>
        ))}
        <a href="#/admin/tenants" className="nav-login">管理</a>
        <a href="#/login" className="nav-login">登录</a>
      </nav>
      <Page />
    </div>
  );
}
