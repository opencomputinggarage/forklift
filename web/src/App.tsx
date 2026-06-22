import { createContext, useContext } from "react";
import { Link, Outlet, useLocation, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, Me } from "./api";
import { Logo } from "./components/Logo";
import { Login } from "./pages/Login";

interface AuthContextValue {
  me: Me;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used below App");
  return ctx;
}

export function App() {
  const queryClient = useQueryClient();
  const meQuery = useQuery({
    queryKey: ["me"],
    queryFn: () => api.me().catch(() => ({ authenticated: false }) as Me),
  });

  const me = meQuery.data;
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["me"] });

  if (meQuery.isLoading) return <div className="login-wrap">Loading…</div>;

  if (!me?.authenticated) return <Login onLogin={refresh} />;

  return (
    <AuthContext.Provider value={{ me }}>
      <div className="app">
        <Sidebar me={me} onLogout={() => api.logout().then(refresh)} />
        <div className="main">
          <Outlet />
        </div>
      </div>
    </AuthContext.Provider>
  );
}

function Sidebar({ me, onLogout }: { me: Me; onLogout: () => void }) {
  const navigate = useNavigate();
  const location = useLocation();
  const canApprove = Boolean(me.admin || me.approver);
  const repoCount = useQuery({
    queryKey: ["sidebar", "repositories", location.pathname],
    queryFn: () => api.listRepositories().then((r) => r.length),
  });
  const pendingCount = useQuery({
    queryKey: ["sidebar", "approvals", location.pathname],
    queryFn: () => api.approvalCount().then((r) => r.count),
    enabled: canApprove,
  });
  const version = useQuery({
    queryKey: ["version"],
    queryFn: () => api.version(),
  });
  const active = (path: string) =>
    location.pathname === path || (path !== "/" && location.pathname.startsWith(`${path}/`));

  return (
    <div className="sidebar">
      <div className="brand-block">
        <Link to="/repositories" className="brand"><Logo /><span className="brand-text">fork<span>lift</span></span></Link>
        {version.data && (
          <span className="brand-version">
            {version.data.version}
            {version.data.commit && version.data.commit !== "none" && (
              <span className="brand-commit"> ({version.data.commit.slice(0, 7)})</span>
            )}
          </span>
        )}
      </div>
      <Link className={`navlink nav-flex${active("/repositories") ? " active" : ""}`} to="/repositories">
        <span>Repositories</span>
        {repoCount.data !== undefined && <span className="count-badge">{repoCount.data}</span>}
      </Link>
      <Link className={`navlink${active("/tokens") ? " active" : ""}`} to="/tokens">Access Tokens</Link>
      {canApprove && (
        <Link className={`navlink nav-flex${active("/approvals") ? " active" : ""}`} to="/approvals">
          <span>Approvals</span>
          {pendingCount.data !== undefined && pendingCount.data > 0 && <span className="count-badge">{pendingCount.data}</span>}
        </Link>
      )}
      {(me.admin || me.auditor) && <Link className={`navlink${active("/users") ? " active" : ""}`} to="/users">Users</Link>}
      {(me.admin || me.auditor) && <Link className={`navlink${active("/roles") ? " active" : ""}`} to="/roles">Roles</Link>}
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
