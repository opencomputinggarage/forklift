import { useEffect, useState } from "react";
import { createFileRoute, Link, Navigate } from "@tanstack/react-router";
import { api, Me, User } from "../../api";
import { useAuth } from "../../authContext";
import { PageDescription, PageHeader, Panel, PanelBody } from "@/components/app-ui/page";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { buttonVariants } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/app-ui/table";

export const Route = createFileRoute("/users/")({
  component: UsersRoute,
});

function UsersRoute() {
  const { me } = useAuth();
  return me.admin || me.auditor ? <Users me={me} /> : <Navigate to="/repositories" replace />;
}

// Admin user directory (read-only). All edits (role mapping, password reset,
// enable/disable, delete) happen on each user's Modify page; creation and its
// initial role assignment happen on /users/new.
export function Users({ me }: { me: Me }) {
  const [users, setUsers] = useState<User[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listUsers().then(setUsers).catch((e) => setError(e.message));
  }, []);

  return (
    <>
      <PageHeader
        title="Users"
        actions={me.admin && <Link className={buttonVariants()} to="/users/new">Create user</Link>}
      />
      <PageDescription>
        Local and OIDC accounts. Open a user to map roles, reset the password, or disable access.
        OIDC users appear automatically at first login.
      </PageDescription>
      {error && <Alert className="mb-4">{error}</Alert>}

      <Panel>
        <PanelBody>
          <h2 className="mb-3 text-base font-semibold">Users</h2>
          <Table>
            <TableHeader>
              <TableRow><TableHead>Username</TableHead><TableHead>Source</TableHead><TableHead>Email</TableHead><TableHead>Roles</TableHead><TableHead>Status</TableHead><TableHead>Last login</TableHead><TableHead /></TableRow>
            </TableHeader>
            <TableBody>
            {users.map((u) => (
              <TableRow key={u.id}>
                <TableCell className="whitespace-nowrap">
                  {u.username}
                  {u.username === me.username && <Badge className="ml-2">you</Badge>}
                </TableCell>
                <TableCell><Badge>{u.source}</Badge></TableCell>
                <TableCell className="text-muted-foreground">{u.email || "-"}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1.5">
                    {u.roles.map((r) => <Link key={r.id} className={buttonVariants({ variant: "outline", size: "xs" })} to="/roles/$id" params={{ id: String(r.id) }}>{r.name}</Link>)}
                    {u.roles.length === 0 && <span className="text-muted-foreground">none</span>}
                  </div>
                </TableCell>
                <TableCell>
                  {u.disabled
                    ? <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><span className="size-2 rounded-full bg-destructive" /> disabled</span>
                    : <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><span className="size-2 rounded-full bg-emerald-400" /> active</span>}
                </TableCell>
                <TableCell className="whitespace-nowrap text-muted-foreground" title={u.last_login_at ?? undefined}>
                  {u.last_login_at ? new Date(u.last_login_at).toLocaleString() : "never"}
                </TableCell>
                <TableCell className="whitespace-nowrap text-right">
                  <Link className={buttonVariants({ variant: "outline" })} to="/users/$id" params={{ id: String(u.id) }}>Modify</Link>
                </TableCell>
              </TableRow>
            ))}
            {users.length === 0 && <TableRow><TableCell colSpan={7} className="text-muted-foreground">No users.</TableCell></TableRow>}
            </TableBody>
          </Table>
        </PanelBody>
      </Panel>
    </>
  );
}
