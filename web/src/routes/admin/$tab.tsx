import { createFileRoute, Link, Navigate, useParams } from "@tanstack/react-router";
import { useAuth } from "@/authContext";
import { PageHeader } from "@/components/app-ui/page";
import { cn } from "@/lib/utils";
import { HAStatusPanel } from "../ha";
import { Receivers } from "../notifications/index";

export const Route = createFileRoute("/admin/$tab")({
  component: AdminRoute,
});

function AdminRoute() {
  const { me } = useAuth();
  // Admin-only: HA topology/failover and notification receivers are operator
  // controls. Non-admins (incl. auditors) are bounced to Repositories.
  return me.admin ? <AdminPage /> : <Navigate to="/repositories" replace />;
}

const TABS = [
  { key: "ha", label: "HA Status" },
  { key: "notifications", label: "Notifications" },
];

function AdminPage() {
  const { tab: routeTab } = useParams({ strict: false }) as { tab?: string };
  const tab = routeTab ?? "ha";

  if (!TABS.some((t) => t.key === tab)) {
    return <Navigate to="/admin/$tab" params={{ tab: "ha" }} replace />;
  }

  return (
    <>
      <PageHeader title="Admin" />

      <nav className="mb-[18px] flex gap-1 border-b border-border max-[760px]:overflow-x-auto max-[760px]:pb-px">
        {TABS.map((t) => (
          <Link key={t.key} className={cn("mb-[-1px] rounded-t-[var(--radius)] border border-transparent border-b-0 px-3.5 py-[9px] text-sm text-muted-foreground hover:bg-muted hover:text-foreground hover:no-underline max-[760px]:whitespace-nowrap", tab === t.key && "border-border border-b-card bg-card text-primary")}
            to="/admin/$tab" params={{ tab: t.key }}>{t.label}</Link>
        ))}
      </nav>

      {tab === "ha" && <HAStatusPanel />}
      {tab === "notifications" && <Receivers />}
    </>
  );
}
