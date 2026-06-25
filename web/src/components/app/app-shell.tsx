import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, Outlet, useNavigate } from "@tanstack/react-router";
import { api, type Me } from "@/api";
import { AuthProvider } from "@/authContext";
import { Login } from "@/components/auth/login";
import { Badge } from "@/components/app-ui/badge";
import { Logo } from "@/components/app/logo";
import { Button } from "@/components/ui/button";
import { openApiQueryOptions } from "@/generated/openapi-query-options";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

export function AppShell() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const meQueryOptions = openApiQueryOptions.getMe();
  const { data: me, isLoading } = useQuery({
    ...meQueryOptions,
    queryFn: () => api.me().catch(() => ({ authenticated: false }) as Me),
    retry: false,
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: meQueryOptions.queryKey });

  if (isLoading) return <div className="flex min-h-screen w-full items-center justify-center">{t("common.loading")}</div>;

  if (!me?.authenticated) {
    return <Login onLogin={refresh} />;
  }

  return (
    <AuthProvider value={{ me }}>
      <TooltipProvider>
        <div className="min-h-screen lg:flex lg:items-start">
          <Sidebar me={me} onLogout={() => api.logout().then(refresh)} />
          <main className="w-full min-w-0 flex-1 px-3 py-4 sm:px-5 sm:py-5 lg:px-8 lg:py-8 xl:px-10">
            <Outlet />
          </main>
        </div>
      </TooltipProvider>
    </AuthProvider>
  );
}

function Sidebar({ me, onLogout }: { me: Me; onLogout: () => void }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const canApprove = Boolean(me.admin || me.approver);
  const { data: repoCount = null } = useQuery({
    ...openApiQueryOptions.listRepositories(),
    select: (repositories) => repositories.length,
  });
  const { data: pendingCount = null } = useQuery({
    ...openApiQueryOptions.getApprovalsCount({ query: { status: "pending" } }),
    select: (result) => result.count,
    enabled: canApprove,
  });
  const { data: version = null } = useQuery({
    queryKey: openApiQueryOptions.getVersion().queryKey,
    queryFn: () => api.version().catch(() => null),
    retry: false,
    staleTime: Infinity,
  });

  const navLinkClass = (active = false) =>
    cn(
      "flex shrink-0 items-center rounded-lg border px-2.5 py-2 text-sm transition-colors hover:no-underline max-sm:px-2",
      active
        ? "border-border bg-muted text-primary"
        : "border-transparent text-muted-foreground hover:border-border hover:bg-muted hover:text-foreground"
    );

  return (
    <aside className="sticky top-0 z-40 flex h-screen w-[var(--fx-sidebar-width)] shrink-0 flex-col gap-1 overflow-y-auto border-r border-border bg-card px-2.5 py-4 shadow-[var(--fx-panel-highlight)] lg:self-start max-lg:h-auto max-lg:w-full max-lg:overflow-visible max-lg:border-r-0 max-lg:border-b max-lg:px-3 max-lg:py-3 max-sm:px-2">
      <div className="px-2 pb-5 max-lg:flex max-lg:items-center max-lg:justify-between max-lg:gap-3 max-lg:pb-3 max-sm:flex-wrap max-sm:items-start max-sm:px-1">
        <Link
          to="/repositories"
          className="flex min-w-0 items-center gap-2.5 text-[22px] font-bold text-foreground hover:no-underline hover:opacity-85 max-sm:text-xl"
        >
          <Logo />
          <span className="truncate">fork<span className="text-primary">lift</span></span>
        </Link>
        {version && (
          <span className="ml-11 mt-0.5 block shrink-0 text-xs font-medium text-muted-foreground max-lg:m-0 max-sm:w-full max-sm:pl-10">
            {version.version}
            {version.commit && version.commit !== "none" && (
              <span className="opacity-65"> ({version.commit.slice(0, 7)})</span>
            )}
          </span>
        )}
      </div>
      <nav className="-mx-1 flex flex-col gap-1 px-1 max-lg:flex-row max-lg:overflow-x-auto max-lg:pb-1 max-lg:[scrollbar-width:none] max-lg:[&::-webkit-scrollbar]:hidden">
        <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/repositories">
          <span>{t("nav.repositories")}</span>
          {repoCount !== null && <Badge className="ml-2 min-w-5 justify-center px-1.5 lg:ml-auto">{repoCount}</Badge>}
        </Link>
        <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/tokens">{t("nav.tokens")}</Link>
        {canApprove && (
          <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/approvals">
            <span>{t("nav.approvals")}</span>
            {pendingCount !== null && pendingCount > 0 && <Badge className="ml-2 min-w-5 justify-center bg-primary text-primary-foreground lg:ml-auto">{pendingCount}</Badge>}
          </Link>
        )}
        {(me.admin || me.auditor) && <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/users">{t("nav.users")}</Link>}
        {(me.admin || me.auditor) && <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/roles">{t("nav.roles")}</Link>}
        {me.admin && <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/admin">{t("nav.admin")}</Link>}
        <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/settings">{t("nav.settings")}</Link>
      </nav>
      <div className="flex-1" />
      <div className="mt-3 flex flex-col gap-1 border-t border-border pt-3 max-lg:mt-2 max-lg:flex-row max-lg:items-center max-lg:justify-between max-lg:gap-3 max-lg:overflow-x-auto max-lg:pt-2 max-sm:flex-wrap max-sm:items-stretch">
        <a className={navLinkClass()} href="/api-docs" target="_blank" rel="noreferrer">{t("nav.apiDocs")}</a>
        <div className="min-w-0 px-2 text-xs text-muted-foreground max-lg:flex max-lg:items-center max-lg:gap-2 max-lg:px-0 max-sm:w-full max-sm:justify-between">
          <div className="truncate">
            {me.username} {me.admin ? `(${t("role.admin")})` : me.auditor ? `(${t("role.auditor")})` : ""}
          </div>
          <Button
            className="mt-2 w-full max-lg:mt-0 max-lg:w-auto max-sm:shrink-0"
            variant="outline"
            type="button"
            onClick={() => { onLogout(); navigate({ to: "/" }); }}
          >
            {t("nav.logout")}
          </Button>
        </div>
      </div>
    </aside>
  );
}
