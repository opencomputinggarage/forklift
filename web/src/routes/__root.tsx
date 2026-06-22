import { useEffect, useState } from "react";
import { createRootRoute, Link, Outlet, useLocation, useNavigate } from "@tanstack/react-router";
import { api, Me } from "../api";
import { AuthProvider } from "../authContext";
import { Login } from "../components/login";
import { Logo } from "../components/logo";
import { Badge } from "@/components/app-ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

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

  if (loading) return <div className="flex min-h-screen w-full items-center justify-center">Loading…</div>;

  if (!me?.authenticated) {
    return <Login onLogin={refresh} />;
  }

  return (
    <AuthProvider value={{ me }}>
      <div className="flex min-h-screen items-start max-md:block">
        <Sidebar me={me} onLogout={() => api.logout().then(refresh)} />
        <div className="min-w-0 flex-1 px-9 py-8 md:max-w-[1180px] max-md:px-4 max-md:py-6">
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

  const navLinkClass = (active = false) =>
    cn(
      "flex items-center rounded-lg border px-2.5 py-2 text-sm transition-colors hover:no-underline",
      active
        ? "border-border bg-muted text-primary"
        : "border-transparent text-muted-foreground hover:border-border hover:bg-muted hover:text-foreground"
    );

  return (
    <div className="sticky top-0 flex h-screen w-[236px] flex-col gap-1 overflow-y-auto border-r border-border bg-card px-2.5 py-4 shadow-[inset_-1px_0_0_rgba(255,255,255,0.02)] max-md:static max-md:h-auto max-md:w-full max-md:border-r-0 max-md:border-b">
      <div className="px-2 pb-5">
        <Link
          to="/repositories"
          className="flex items-center gap-2.5 text-[22px] font-bold text-foreground hover:no-underline hover:opacity-85"
        >
          <Logo />
          <span className="brand-text">fork<span>lift</span></span>
        </Link>
        {version && (
          <span className="ml-11 mt-0.5 block text-xs font-medium text-muted-foreground">
            {version.version}
            {version.commit && version.commit !== "none" && (
              <span className="opacity-65"> ({version.commit.slice(0, 7)})</span>
            )}
          </span>
        )}
      </div>
      <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/repositories">
        <span>Repositories</span>
        {repoCount !== null && <Badge className="ml-auto min-w-5 justify-center px-1.5">{repoCount}</Badge>}
      </Link>
      <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/tokens">Access Tokens</Link>
      {canApprove && (
        <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/approvals">
          <span>Approvals</span>
          {pendingCount !== null && pendingCount > 0 && <Badge className="ml-auto min-w-5 justify-center bg-primary text-primary-foreground">{pendingCount}</Badge>}
        </Link>
      )}
      {(me.admin || me.auditor) && <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/users">Users</Link>}
      {(me.admin || me.auditor) && <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/roles">Roles</Link>}
      <div className="flex-1" />
      <a className={navLinkClass()} href="/api-docs" target="_blank" rel="noreferrer">API Docs ↗</a>
      <div className="border-t border-border px-2 py-3 text-xs text-muted-foreground">
        <div>{me.username} {me.admin ? "(admin)" : me.auditor ? "(auditor)" : ""}</div>
        <Button
          className="mt-2 w-full"
          variant="outline"
          type="button"
          onClick={() => { onLogout(); navigate({ to: "/" }); }}
        >
          Log Out
        </Button>
      </div>
    </div>
  );
}
