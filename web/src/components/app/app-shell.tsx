import type React from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, Outlet, useNavigate } from "@tanstack/react-router";
import {
  Bell,
  BookOpen,
  Boxes,
  ClipboardCheck,
  KeyRound,
  LogOut,
  Settings,
  Shield,
  SlidersHorizontal,
  UserRound,
  UsersRound,
} from "lucide-react";
import { api, type Me } from "@/api";
import { AuthProvider } from "@/authContext";
import { Login } from "@/components/auth/login";
import { Logo } from "@/components/app/logo";
import { Badge } from "@/components/ui/badge";
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
        <div className="min-h-dvh bg-background lg:flex lg:items-start">
          <Sidebar me={me} onLogout={() => api.logout().then(refresh)} />
          <main className="w-full min-w-0 flex-1 px-[var(--fx-main-gutter-x)] py-[var(--fx-main-gutter-y)] max-sm:px-3 max-sm:py-4">
            <div className="mx-auto w-full max-w-[var(--fx-content-max)]">
              <Outlet />
            </div>
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
      "group flex shrink-0 items-center gap-2 rounded-md border px-2 py-1.5 text-[13px] leading-5 transition-colors hover:no-underline max-sm:px-2",
      active
        ? "border-[var(--fx-border-subtle)] bg-[var(--fx-surface-selected)] text-foreground shadow-[inset_2px_0_0_var(--fx-accent)]"
        : "border-transparent text-muted-foreground hover:border-[var(--fx-border-subtle)] hover:bg-[var(--fx-surface-hover)] hover:text-foreground"
    );

  return (
    <aside className="sticky top-0 z-40 flex h-dvh w-[var(--fx-sidebar-width)] shrink-0 flex-col gap-1 overflow-y-auto border-r border-[var(--fx-border-subtle)] bg-[var(--fx-surface-panel)] px-2.5 py-3 shadow-[var(--fx-panel-highlight)] lg:self-start max-lg:h-auto max-lg:w-full max-lg:overflow-visible max-lg:border-r-0 max-lg:border-b max-lg:px-3 max-lg:py-2 max-sm:px-2">
      <div className="px-1.5 pb-4 max-lg:flex max-lg:items-center max-lg:justify-between max-lg:gap-3 max-lg:pb-2 max-sm:px-1">
        <Link
          to="/repositories"
          className="flex min-w-0 items-center gap-2.5 text-[20px] font-bold text-foreground hover:no-underline hover:opacity-85 max-sm:text-lg"
        >
          <Logo />
          <span className="truncate">fork<span className="text-primary">lift</span></span>
        </Link>
        {version && (
          <span className="ml-11 mt-0.5 block shrink-0 text-xs font-medium text-muted-foreground max-lg:m-0 max-sm:hidden">
            {version.version}
            {version.commit && version.commit !== "none" && (
              <span className="opacity-65"> ({version.commit.slice(0, 7)})</span>
            )}
          </span>
        )}
      </div>
      <nav className="-mx-1 flex flex-col gap-0.5 px-1 max-lg:flex-row max-lg:overflow-x-auto max-lg:pb-1 max-lg:[scrollbar-width:none] max-lg:[&::-webkit-scrollbar]:hidden">
        <NavGroup title={t("nav.group.workspace")}>
          <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/repositories">
            <Boxes className="size-4 opacity-75 group-hover:opacity-100" aria-hidden="true" />
            <span>{t("nav.repositories")}</span>
            {repoCount !== null && <Badge variant="outline" className="ml-2 min-w-5 justify-center px-1.5 lg:ml-auto">{repoCount}</Badge>}
          </Link>
          <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/tokens">
            <KeyRound className="size-4 opacity-75 group-hover:opacity-100" aria-hidden="true" />
            {t("nav.tokens")}
          </Link>
          {canApprove && (
            <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/approvals">
              <ClipboardCheck className="size-4 opacity-75 group-hover:opacity-100" aria-hidden="true" />
              <span>{t("nav.approvals")}</span>
              {pendingCount !== null && pendingCount > 0 && <Badge className="ml-2 min-w-5 justify-center bg-primary text-primary-foreground lg:ml-auto">{pendingCount}</Badge>}
            </Link>
          )}
        </NavGroup>
        {(me.admin || me.auditor) && (
          <NavGroup title={t("nav.group.access")}>
            <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/users">
              <UserRound className="size-4 opacity-75 group-hover:opacity-100" aria-hidden="true" />
              {t("nav.users")}
            </Link>
            <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/roles">
              <UsersRound className="size-4 opacity-75 group-hover:opacity-100" aria-hidden="true" />
              {t("nav.roles")}
            </Link>
          </NavGroup>
        )}
        <NavGroup title={t("nav.group.system")}>
          <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/settings">
            <Settings className="size-4 opacity-75 group-hover:opacity-100" aria-hidden="true" />
            {t("nav.settings")}
          </Link>
        </NavGroup>
        {me.admin && (
          <NavGroup title={t("nav.group.admin")}>
            <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/admin">
              <Shield className="size-4 opacity-75 group-hover:opacity-100" aria-hidden="true" />
              {t("nav.admin")}
            </Link>
            <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/notifications">
              <Bell className="size-4 opacity-75 group-hover:opacity-100" aria-hidden="true" />
              {t("nav.notifications")}
            </Link>
            <Link className={navLinkClass()} activeProps={{ className: navLinkClass(true) }} to="/ha">
              <SlidersHorizontal className="size-4 opacity-75 group-hover:opacity-100" aria-hidden="true" />
              {t("nav.ha")}
            </Link>
          </NavGroup>
        )}
      </nav>
      <div className="flex-1" />
      <div className="mt-3 flex flex-col gap-1 border-t border-[var(--fx-border-subtle)] pt-3 max-lg:mt-2 max-lg:flex-row max-lg:items-center max-lg:justify-between max-lg:gap-3 max-lg:overflow-x-auto max-lg:pt-2 max-sm:gap-2">
        <a className={navLinkClass()} href="/api-docs" target="_blank" rel="noreferrer">
          <BookOpen className="size-4 opacity-75 group-hover:opacity-100" aria-hidden="true" />
          {t("nav.apiDocs")}
        </a>
        <div className="min-w-0 px-2 text-xs text-muted-foreground max-lg:flex max-lg:items-center max-lg:gap-2 max-lg:px-0 max-sm:ml-auto max-sm:w-auto">
          <div className="truncate">
            {me.username} {me.admin ? `(${t("role.admin")})` : me.auditor ? `(${t("role.auditor")})` : ""}
          </div>
          <Button
            className="mt-2 w-full gap-1.5 max-lg:mt-0 max-lg:w-auto max-sm:shrink-0"
            variant="outline"
            type="button"
            onClick={() => { onLogout(); navigate({ to: "/" }); }}
          >
            <LogOut className="size-3.5" aria-hidden="true" />
            {t("nav.logout")}
          </Button>
        </div>
      </div>
    </aside>
  );
}

function NavGroup({ title, children }: { title: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="contents lg:flex lg:flex-col lg:gap-0.5 lg:pt-3 first:lg:pt-0" aria-label={typeof title === "string" ? title : undefined}>
      <div className="px-2 pb-1 text-[11px] font-medium uppercase leading-4 text-[var(--fx-text-subtle)] max-lg:hidden">
        {title}
      </div>
      {children}
    </section>
  );
}
