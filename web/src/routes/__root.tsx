import { useEffect, useState } from "react";
import { createRootRoute, Link, Outlet, useLocation, useNavigate } from "@tanstack/react-router";
import { api, Me } from "../api";
import { AuthProvider } from "../authContext";
import { Login } from "../components/Login";
import { Logo } from "../components/Logo";

export const Route = createRootRoute({
  component: AppShell,
});

function AppShell() {
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = () => api.me().then(setMe).catch(() => setMe({ authenticated: false }));

  useEffect(() => {
    refresh().finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="login-wrap">Loading…</div>;

  if (!me?.authenticated) {
    return <Login onLogin={refresh} />;
  }

  return (
    <AuthProvider value={{ me }}>
      <div className="app">
        <Sidebar me={me} onLogout={() => api.logout().then(refresh)} />
        <div className="main">
          <Outlet />
        </div>
      </div>
    </AuthProvider>
  );
}

function Sidebar({ me, onLogout }: { me: Me; onLogout: () => void }) {
  const navigate = useNavigate();
  const location = useLocation();
  const [repoCount, setRepoCount] = useState<number | null>(null);
  const [pendingCount, setPendingCount] = useState<number | null>(null);
  const [version, setVersion] = useState<{ version: string; commit: string } | null>(null);

  const canApprove = Boolean(me.admin || me.approver);
  useEffect(() => {
    api.listRepositories().then((r) => setRepoCount(r.length)).catch(() => setRepoCount(null));
    if (canApprove) {
      api.approvalCount().then((r) => setPendingCount(r.count)).catch(() => setPendingCount(null));
    }
  }, [location.pathname, canApprove]);

  useEffect(() => {
    api.version().then((v) => setVersion(v)).catch(() => setVersion(null));
  }, []);

  return (
    <div className="sidebar">
      <div className="brand-block">
        <Link to="/repositories" className="brand"><Logo /><span className="brand-text">fork<span>lift</span></span></Link>
        {version && (
          <span className="brand-version">
            {version.version}
            {version.commit && version.commit !== "none" && (
              <span className="brand-commit"> ({version.commit.slice(0, 7)})</span>
            )}
          </span>
        )}
      </div>
      <Link className="navlink nav-flex" activeProps={{ className: "navlink nav-flex active" }} to="/repositories">
        <span>Repositories</span>
        {repoCount !== null && <span className="count-badge">{repoCount}</span>}
      </Link>
      <Link className="navlink" activeProps={{ className: "navlink active" }} to="/tokens">Access Tokens</Link>
      {canApprove && (
        <Link className="navlink nav-flex" activeProps={{ className: "navlink nav-flex active" }} to="/approvals">
          <span>Approvals</span>
          {pendingCount !== null && pendingCount > 0 && <span className="count-badge">{pendingCount}</span>}
        </Link>
      )}
      {(me.admin || me.auditor) && <Link className="navlink" activeProps={{ className: "navlink active" }} to="/users">Users</Link>}
      {(me.admin || me.auditor) && <Link className="navlink" activeProps={{ className: "navlink active" }} to="/roles">Roles</Link>}
      <div className="spacer" />
      <a className="navlink" href="/api-docs" target="_blank" rel="noreferrer">API Docs ↗</a>
      <div className="userbox">
        <div>{me.username} {me.admin ? "(admin)" : me.auditor ? "(auditor)" : ""}</div>
        <button type="button" className="btn secondary logout-btn"
          onClick={() => { onLogout(); navigate({ to: "/" }); }}>Log Out</button>
      </div>
    </div>
  );
}
