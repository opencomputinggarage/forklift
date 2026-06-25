import { useEffect, useState } from "react";
import { createFileRoute, Link, Navigate } from "@tanstack/react-router";
import { api, Me, User } from "@/api";
import { useAuth } from "@/authContext";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type ColumnDef } from "@/components/app-ui/table";

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
  const [users, setUsers] = useState<User[]>([]);
  const [error, setError] = useState("");
  const columns: ColumnDef<User>[] = [
    {
      header: "Username",
      cell: ({ row }) => {
        const user = row.original;
        return (
          <span className="whitespace-nowrap">
            {user.username}
            {user.username === me.username && <Badge className="ml-2">you</Badge>}
          </span>
        );
      },
    },
    {
      header: "Source",
      cell: ({ row }) => <Badge>{row.original.source}</Badge>,
    },
    {
      header: "Email",
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.email || "-"}</span>,
    },
    {
      header: "Roles",
      cell: ({ row }) => (
        <div className="flex flex-wrap gap-1.5">
          {row.original.roles.map((r) => (
            <Button
              key={r.id}
              variant="outline"
              size="xs"
              render={<Link to="/access/roles/$id" params={{ id: String(r.id) }} />}
              nativeButton={false}
            >
              {r.name}
            </Button>
          ))}
          {row.original.roles.length === 0 && <span className="text-muted-foreground">none</span>}
        </div>
      ),
    },
    {
      header: "Status",
      cell: ({ row }) => row.original.disabled
        ? <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><span className="size-2 rounded-full bg-destructive" /> disabled</span>
        : <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><span className="size-2 rounded-full bg-[var(--fx-success)]" /> active</span>,
    },
    {
      header: "Last login",
      cell: ({ row }) => (
        <span className="whitespace-nowrap text-muted-foreground" title={row.original.last_login_at ?? undefined}>
          {row.original.last_login_at ? new Date(row.original.last_login_at).toLocaleString() : "never"}
        </span>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="text-right">
          <Button
            variant="outline"
            render={<Link to="/access/users/$id" params={{ id: String(row.original.id) }} />}
            nativeButton={false}
          >
            Modify
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
        title="Users"
        actions={me.admin && (
          <Button render={<Link to="/access/users/new" />} nativeButton={false}>
            Create user
          </Button>
        )}
      />
      <PageDescription>
        Local and OIDC accounts. Open a user to map roles, reset the password, or disable access.
        OIDC users appear automatically at first login.
      </PageDescription>
      {error && <Alert className="mb-4">{error}</Alert>}

      <DataTable columns={columns} data={users} empty="No users." />
    </>
  );
}
