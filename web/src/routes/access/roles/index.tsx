import { useEffect, useState } from "react";
import { createFileRoute, Link, Navigate } from "@tanstack/react-router";
import { api, Me, Role } from "@/api";
import { useAuth } from "@/authContext";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type ColumnDef } from "@/components/app-ui/table";

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
  const [roles, setRoles] = useState<Role[]>([]);
  const [error, setError] = useState("");
  const columns: ColumnDef<Role>[] = [
    {
      header: "Role",
      cell: ({ row }) => row.original.name,
    },
    {
      header: "Source",
      cell: ({ row }) => (
        <Badge title={row.original.managed
          ? "Managed by the declarative RBAC policy and not editable in the UI."
          : "Created in the UI or API and editable here."}>
          {row.original.managed ? "managed" : "local"}
        </Badge>
      ),
    },
    {
      header: "Description",
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.description || "-"}</span>,
    },
    {
      header: "Users",
      cell: ({ row }) => row.original.user_count,
    },
    {
      header: "Permissions",
      cell: ({ row }) => (
        <div className="flex flex-wrap gap-1.5">
          {row.original.permissions.map((p) => (
            <Badge key={p.id} className="font-mono">
              {p.repo_pattern}: {p.actions.join(",")}
            </Badge>
          ))}
          {row.original.permissions.length === 0 && <span className="text-muted-foreground">none</span>}
        </div>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="text-right">
          <Button
            variant="outline"
            render={<Link to="/access/roles/$id" params={{ id: String(row.original.id) }} />}
            nativeButton={false}
          >
            Modify
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
        title="Roles"
        actions={me.admin && (
          <Button render={<Link to="/access/roles/new" />} nativeButton={false}>
            Create role
          </Button>
        )}
      />
      <PageDescription>
        Bundle repository permissions (read, write, delete, approve, audit, admin) over name patterns.
        Open a role to map permissions; roles are assigned to users on each user's detail page.
      </PageDescription>
      {error && <Alert className="mb-4">{error}</Alert>}

      <DataTable columns={columns} data={roles} empty="No roles yet. Create one to grant repository access." />
    </>
  );
}
