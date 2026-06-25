import { useEffect, useState } from "react";
import { createFileRoute, Link, Navigate } from "@tanstack/react-router";
import { api, Me, Role } from "@/api";
import { useAuth } from "@/authContext";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrap,
} from "@/components/app-ui/table";

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

      <Card size="sm" className="mb-4">
        <CardContent>
          <TableWrap>
          <Table>
            <TableHeader>
              <TableRow><TableHead>Role</TableHead><TableHead>Source</TableHead><TableHead>Description</TableHead><TableHead>Users</TableHead><TableHead>Permissions</TableHead><TableHead /></TableRow>
            </TableHeader>
            <TableBody>
            {roles.map((r) => (
              <TableRow key={r.id}>
                <TableCell>{r.name}</TableCell>
                <TableCell>
                  <Badge title={r.managed
                    ? "Managed by the declarative RBAC policy and not editable in the UI."
                    : "Created in the UI or API and editable here."}>
                    {r.managed ? "managed" : "local"}
                  </Badge>
                </TableCell>
                <TableCell className="text-muted-foreground">{r.description || "-"}</TableCell>
                <TableCell>{r.user_count}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1.5">
                    {r.permissions.map((p) => (
                      <Badge key={p.id} className="font-mono">
                        {p.repo_pattern}: {p.actions.join(",")}
                      </Badge>
                    ))}
                    {r.permissions.length === 0 && <span className="text-muted-foreground">none</span>}
                  </div>
                </TableCell>
                <TableCell className="whitespace-nowrap text-right">
                  <Button
                    variant="outline"
                    render={<Link to="/access/roles/$id" params={{ id: String(r.id) }} />}
                    nativeButton={false}
                  >
                    Modify
                  </Button>
                </TableCell>
              </TableRow>
            ))}
            {roles.length === 0 && <TableRow><TableCell colSpan={6} className="text-muted-foreground">No roles yet. Create one to grant repository access.</TableCell></TableRow>}
            </TableBody>
          </Table>
          </TableWrap>
        </CardContent>
      </Card>
    </>
  );
}
