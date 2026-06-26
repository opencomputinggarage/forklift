import { useEffect, useState } from "react";
import { createFileRoute, Navigate, useNavigate } from "@tanstack/react-router";
import { api, Me, User } from "@/api";
import { useAuth } from "@/authContext";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type ColumnDef } from "@/components/app-ui/table";
import { useTranslation } from "@/lib/i18n";

export const Route = createFileRoute("/access/users/")({
  component: UsersRoute,
});

function UsersRoute() {
  const { me } = useAuth();
  return me.admin || me.auditor ? <Users me={me} /> : <Navigate to="/workspace/repositories" replace />;
}

// Admin user directory (read-only). All edits (role mapping, password reset,
// enable/disable, delete) happen on each user's Modify page; creation and its
// initial role assignment happen on /access/users/new.
export function Users({ me }: { me: Me }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [users, setUsers] = useState<User[]>([]);
  const [error, setError] = useState("");
  const columns: ColumnDef<User>[] = [
    {
      header: t("common.username"),
      cell: ({ row }) => {
        const user = row.original;
        return (
          <span className="whitespace-nowrap">
            {user.username}
            {user.username === me.username && <Badge className="ml-2">{t("common.you")}</Badge>}
          </span>
        );
      },
    },
    {
      header: t("common.source"),
      cell: ({ row }) => <Badge>{row.original.source}</Badge>,
    },
    {
      header: t("common.email"),
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.email || "-"}</span>,
    },
    {
      header: t("common.roles"),
      cell: ({ row }) => (
        <div className="flex flex-wrap gap-1.5">
          {row.original.roles.map((r) => (
            <Button
              key={r.id}
              variant="outline"
              size="xs"
              onClick={() => navigate({ to: "/access/roles/$id", params: { id: String(r.id) } })}
            >
              {r.name}
            </Button>
          ))}
          {row.original.roles.length === 0 && <span className="text-muted-foreground">{t("common.none")}</span>}
        </div>
      ),
    },
    {
      header: t("common.status"),
      cell: ({ row }) => row.original.disabled
        ? <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><span className="size-2 rounded-full bg-destructive" /> {t("common.status.disabled")}</span>
        : <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><span className="size-2 rounded-full bg-[var(--fx-success)]" /> {t("common.status.active")}</span>,
    },
    {
      header: t("common.last-login"),
      cell: ({ row }) => (
        <span className="whitespace-nowrap text-muted-foreground" title={row.original.last_login_at ?? undefined}>
          {row.original.last_login_at ? new Date(row.original.last_login_at).toLocaleString() : t("common.never")}
        </span>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="text-right">
          <Button
            variant="outline"
            onClick={() => navigate({ to: "/access/users/$id", params: { id: String(row.original.id) } })}
          >
            {t("common.modify")}
          </Button>
        </div>
      ),
    },
  ];

  useEffect(() => {
    api.listUsers().then(setUsers).catch((e) => setError(e.message));
  }, []);

  return (
    <>
      <PageHeader
        title={t("common.users")}
        actions={me.admin && (
          <Button onClick={() => navigate({ to: "/access/users/new" })}>
            {t("user.create")}
          </Button>
        )}
      />
      <PageDescription>
        {t("user.list-description")}
      </PageDescription>
      {error && <Alert className="mb-4">{error}</Alert>}

      <DataTable columns={columns} data={users} empty={t("user.empty")} />
    </>
  );
}
