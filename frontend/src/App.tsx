// ALFQ App — hash-based routing, zero external dependencies
import React, { useState, useEffect } from "react";
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
import Tenants from "./pages/Tenants";
import Users from "./pages/Users";
import Settings from "./pages/Settings";

const routes: Record<string, () => React.ReactNode> = {
  "#/": Dashboard,
  "#/login": Login,
  "#/accounts": Accounts,
  "#/orders": Orders,
  "#/positions": Positions,
  "#/risk": RiskRules,
  "#/strategies": Strategies,
  "#/backtest": Backtest,
  "#/assistant": AIChat,
  "#/audit": Audit,
  "#/notifications": Notifications,
  "#/tenants": Tenants,
  "#/users": Users,
  "#/settings": Settings,
};

const navItems: [string, string][] = [
  ["#/", "仪表盘"],
  ["#/accounts", "账户"],
  ["#/orders", "订单"],
  ["#/positions", "持仓"],
  ["#/risk", "风控"],
  ["#/strategies", "策略"],
  ["#/backtest", "回测"],
  ["#/assistant", "AI助手"],
  ["#/audit", "审计"],
  ["#/admin", "管理"],
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
  const Page = routes[hash] || Dashboard;

  return (
    <div>
      <nav className="navbar">
        <a href="#/" className="navbar-brand">ALFQ</a>
        {navItems.map(([path, label]) => (
          <a key={path} href={path} className={`nav-link${hash === path ? " active" : ""}`}>{label}</a>
        ))}
        <a href="#/login" className="nav-login">登录</a>
      </nav>
      <Page />
    </div>
  );
}
