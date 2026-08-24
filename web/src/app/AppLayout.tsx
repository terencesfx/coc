import { useEffect, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { QuickDiceDrawer } from "./QuickDiceDrawer";

export function AppLayout() {
  const { account, logout } = useAuth();
  const location = useLocation();
  const characterID = location.pathname.match(/^\/characters\/([^/]+)$/)?.[1];
  const [diceOpen, setDiceOpen] = useState(false);
  useEffect(() => {
    setDiceOpen(false);
  }, [location.pathname]);
  return (
    <div className="app-shell">
      <header className="topbar">
        <NavLink className="brand" to={account?.role === "admin" ? "/admin/accounts" : "/"}>
          COC7版人物卡
        </NavLink>
        <div className="account-menu">
          <span>{account?.displayName}</span>
          <button
            className="text-button"
            type="button"
            onClick={() => void logout()}
          >
            退出
          </button>
        </div>
      </header>
      <aside className="sidebar">
        <nav aria-label="主导航">
          {account?.role === "admin" ? <>
            <NavItem to="/admin/accounts" label="账号管理" />
            <NavItem to="/admin/system" label="系统维护" />
          </> : <>
            <NavItem to="/" label="首页" end />
            <NavItem to="/campaigns" label="团本" />
            <NavItem to="/characters" label="人物卡档案" />
            <NavItem to="/rolls" label="投骰记录" />
          </>}
          <NavItem to="/change-password" label="修改密码" />
        </nav>
      </aside>
      <main className="content">
        <Outlet />
      </main>
      {characterID && <>
        <button
          className="dice-fab"
          type="button"
          aria-label="打开当前人物卡的组合骰"
          title="人物卡组合骰"
          onClick={() => setDiceOpen(true)}
        >
          <span aria-hidden="true">◈</span>
        </button>
        <QuickDiceDrawer open={diceOpen} onClose={() => setDiceOpen(false)} characterID={characterID} />
      </>}
    </div>
  );
}

function NavItem({
  to,
  label,
  end = false,
}: {
  to: string;
  label: string;
  end?: boolean;
}) {
  return (
    <NavLink
      className={({ isActive }) =>
        `nav-link${isActive ? " nav-link--active" : ""}`
      }
      to={to}
      end={end}
    >
      {label}
    </NavLink>
  );
}
