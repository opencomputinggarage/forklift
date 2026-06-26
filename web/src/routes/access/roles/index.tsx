import { useEffect, useState } from "react";
import { createFileRoute, Navigate, useNavigate } from "@tanstack/react-router";
import { api, Me, Role } from "@/api";
import { useAuth } from "@/authContext";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type ColumnDef } from "@/components/app-ui/table";
import { useTranslation } from "@/lib/i18n";

export const Route = createFileRoute("/access/roles/")({
  component: RolesRoute,
});

function RolesRoute() {
  const { me } = useAuth();
  return me.admin || me.auditor ? <Roles me={me} /> : <Navigate to="/workspace/repositories" replace />;
}

// Admin role directory (read-only). Roles and their permissions are defined on
// /access/roles/new; this page only displays them.
export function Roles({ me }: { me: Me }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [roles, setRoles] = useState<Role[]>([]);
  const [error, setError] = useState("");
  const columns: ColumnDef<Role>[] = [
    {
      header: t("common.role"),
      cell: ({ row }) => row.original.name,
    },
    {
      header: t("common.source"),
      cell: ({ row }) => (
        <Badge title={row.original.managed
          ? "Managed by the declarative RBAC policy and not editable in the UI."
          : "Created in the UI or API and editable here."}>
          {row.original.managed ? t("common.status.managed") : t("common.status.local")}
        </Badge>
      ),
    },
    {
      header: t("common.description"),
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.description || "-"}</span>,
    },
    {
      header: t("common.users"),
      cell: ({ row }) => row.original.user_count,
    },
    {
      header: t("common.permissions"),
      cell: ({ row }) => (
        <div className="flex flex-wrap gap-1.5">
          {row.original.permissions.map((p) => (
            <Badge key={p.id} className="font-mono">
              {p.repo_pattern}: {p.actions.join(",")}
            </Badge>
          ))}
          {row.original.permissions.length === 0 && <span className="text-muted-foreground">{t("common.none")}</span>}
        </div>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="text-right">
          <Button
            variant="outline"
            onClick={() => navigate({ to: "/access/roles/$id", params: { id: String(row.original.id) } })}
          >
            {t("common.modify")}
          </Button>
        </div>
      ),
    },
  ];

  useEffect(() => {
    api.listRoles().then(setRoles).catch((e) => setError(e.message));
  }, []);

  return (
    <>
      <PageHeader
        title={t("common.roles")}
        actions={me.admin && (
          <Button onClick={() => navigate({ to: "/access/roles/new" })}>
            {t("role.create")}
          </Button>
        )}
      />
      <PageDescription>
        {t("role.list-description")}
      </PageDescription>
      {error && <Alert className="mb-4">{error}</Alert>}

      <DataTable columns={columns} data={roles} empty={t("role.empty")} />
    </>
  );
}
