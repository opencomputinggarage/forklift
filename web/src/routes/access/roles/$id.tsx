import { useEffect, useState } from "react";
import { createFileRoute, Link, Navigate, useNavigate, useParams } from "@tanstack/react-router";
import { LockKeyhole, X } from "lucide-react";
import { api, Me, Role, User } from "@/api";
import { useAuth } from "@/authContext";
import { Combobox } from "@/components/inputs/combobox";
import { ConfirmModal } from "@/components/overlays/confirm-modal";
import { Alert } from "@/components/app-ui/alert";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import { CountBadge, StateBadge } from "@/components/app-ui/status-badge";
import { PermissionBadge, RoleBadge } from "@/components/app-ui/action-badge";
import { SourceBadge } from "@/components/app-ui/source-badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrap,
} from "@/components/app-ui/table";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";

export const Route = createFileRoute("/access/roles/$id")({
  component: RoleModifyRoute,
});

function RoleModifyRoute() {
  const { me } = useAuth();
  return me.admin || me.auditor ? <RoleModify me={me} /> : <Navigate to="/workspace/repositories" replace />;
}

const ACTIONS = ["read", "write", "delete", "approve", "audit", "admin"];

// Per-role modify page: permission mapping, assigned users, and the danger zone
// (delete). The Roles list is read-only; all edits happen here. The page is
// read-only (no add/remove permission, no delete) for an auditor and for managed
// roles, which are owned by the chart's declarative RBAC policy.
export function RoleModify({ me }: { me: Me }) {
  const { id } = useParams({ strict: false }) as { id?: string };
  const navigate = useNavigate();
  const roleId = Number(id);
  const [role, setRole] = useState<Role | null>(null);
  const [members, setMembers] = useState<User[]>([]);
  const [error, setError] = useState("");

  const load = () =>
    Promise.all([api.listRoles(), api.listUsers()])
      .then(([roles, users]) => {
        const r = roles.find((x) => x.id === roleId) ?? null;
        setRole(r);
        setMembers(users.filter((u) => u.roles.some((ur) => ur.id === roleId)));
        if (!r) setError("Role not found.");
      })
      .catch((e) => setError(e.message));
  useEffect(() => { load(); /* eslint-disable-next-line */ }, [roleId]);

  if (error && !role) return <Alert className="my-2.5">{error}</Alert>;
  if (!role) return <div className="text-sm text-muted-foreground">Loading…</div>;

  const run = (p: Promise<unknown>) => {
    setError("");
    p.then(load).catch((e) => setError((e as Error).message));
  };

  // Managed roles are reconciled from the chart's declarative RBAC policy and are
  // read-only via the API. Gate every edit control on !role.managed so an admin
  // never sees a button that would only return a 409; the backend still enforces
  // this regardless of the UI.
  const editable = !!me.admin && !role.managed;

  return (
    <>
      <PageHeader
        title={role.name}
        actions={
        <Button render={<Link to="/access/roles" />} nativeButton={false} variant="outline">
          Back to roles
        </Button>
        }
      />
      {role.description && <PageDescription>{role.description}</PageDescription>}
      {role.managed && (
        <Card size="sm" className="mb-4 border-primary/70">
          <CardContent>
          <h2 className="mb-2 flex items-center gap-2 text-base font-semibold">
            <LockKeyhole className="size-4 text-primary" aria-hidden="true" />
            Managed role
          </h2>
          <p className="m-0 text-sm leading-relaxed text-muted-foreground">
            This role was configured by a Forklift administrator in the declarative RBAC policy so it cannot be edited here. To change its permissions or delete it ask an administrator to update the policy file and restart forklift.
          </p>
          </CardContent>
        </Card>
      )}
      {error && <Alert className="mb-4">{error}</Alert>}

      <PermissionsPanel role={role} run={run} canWrite={editable} />
      <AssignedUsersPanel members={members} />
      {editable && <DangerPanel role={role} onDeleted={() => navigate({ to: "/access/roles" })} onError={setError} />}
    </>
  );
}

// AssignedUsersPanel lists the users that currently hold this role. Assignment
// itself is managed on each user's detail page, so this is read-only with links.
function AssignedUsersPanel({ members }: { members: User[] }) {
  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-4 text-base font-semibold">
        Assigned users <CountBadge className="ml-1.5">{members.length}</CountBadge>
      </h2>
      {members.length === 0
        ? <p className="m-0 text-sm text-muted-foreground">No users have this role. Assign it from a user's detail page.</p>
        : (
          // Same column structure and order as the Users page; the username
          // links to that user's detail page.
          <TableWrap>
          <Table>
            <TableHeader>
              <TableRow><TableHead>Username</TableHead><TableHead>Source</TableHead><TableHead>Email</TableHead><TableHead>Roles</TableHead><TableHead>Status</TableHead><TableHead>Last login</TableHead></TableRow>
            </TableHeader>
            <TableBody>
              {members.map((u) => (
                <TableRow key={u.id}>
                  <TableCell className="whitespace-nowrap"><Link to="/access/users/$id" params={{ id: String(u.id) }}>{u.username}</Link></TableCell>
                  <TableCell><SourceBadge source={u.source} /></TableCell>
                  <TableCell className="text-muted-foreground">{u.email || "-"}</TableCell>
                  <TableCell>
                    <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap flex-wrap gap-1.5">
                      {u.roles.map((r) => (
                        <RoleBadge
                          key={r.id}
                          render={<Link to="/access/roles/$id" params={{ id: String(r.id) }} />}
                        >
                          {r.name}
                        </RoleBadge>
                      ))}
                      {u.roles.length === 0 && <span className="text-muted-foreground">none</span>}
                    </div>
                  </TableCell>
                  <TableCell>
                    {u.disabled
                      ? <StateBadge state="disabled">disabled</StateBadge>
                      : <StateBadge state="active">active</StateBadge>}
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-muted-foreground" title={u.last_login_at ?? undefined}>
                    {u.last_login_at ? new Date(u.last_login_at).toLocaleString() : "never"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          </TableWrap>
        )}
      </CardContent>
    </Card>
  );
}

function PermissionsPanel({ role, run, canWrite }: { role: Role; run: (p: Promise<unknown>) => void; canWrite: boolean }) {
  const [pattern, setPattern] = useState("");
  const [actions, setActions] = useState<string[]>(["read"]);
  const [repoOptions, setRepoOptions] = useState<string[]>(["*"]);
  const [repoTypes, setRepoTypes] = useState<Record<string, string>>({});

  useEffect(() => {
    api.listRepositoryNames()
      .then((repos) => {
        setRepoOptions(["*", ...repos.map((r) => r.name)]);
        setRepoTypes(Object.fromEntries(repos.map((r) => [r.name, `${r.format} · ${r.type}`])));
      })
      .catch(() => setRepoOptions(["*"]));
  }, []);

  const toggle = (a: string) =>
    setActions((cur) => cur.includes(a) ? cur.filter((x) => x !== a) : [...cur, a]);

  const add = () => {
    run(api.addPermission(role.id, { repo_pattern: pattern.trim(), actions }));
    setPattern("");
    setActions(["read"]);
  };

  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-4 text-base font-semibold">Permissions</h2>
      <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap flex-wrap gap-1.5">
        {role.permissions.map((p) => (
          <PermissionBadge key={p.id} className="gap-1">
            <span>{p.repo_pattern}: {p.actions.join(",")}</span>
            {canWrite && (
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                className="-mr-1 size-4 rounded-full text-muted-foreground hover:bg-background/40 hover:text-foreground"
                title="Remove permission"
                onClick={() => run(api.deletePermission(role.id, p.id))}
              >
                <X className="size-3" aria-hidden="true" />
                <span className="sr-only">Remove permission</span>
              </Button>
            )}
          </PermissionBadge>
        ))}
        {role.permissions.length === 0 && <span className="text-sm text-muted-foreground">No permissions granted.</span>}
      </div>
      {canWrite && (
        <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap mt-4 flex-wrap items-stretch gap-2 max-sm:flex-col">
          <Combobox className="w-full sm:w-[200px]" value={pattern} onChange={setPattern}
            options={repoOptions} hints={repoTypes} placeholder="repo pattern (* or maven-*)" />
          {ACTIONS.map((a) => (
            <label key={a} className="flex items-center gap-2 text-xs">
              <Checkbox checked={actions.includes(a)} onCheckedChange={() => toggle(a)} />
              <span>{a}</span>
            </label>
          ))}
          <Button variant="outline" type="button"
            disabled={!pattern.trim() || actions.length === 0} onClick={add}>Add</Button>
        </div>
      )}
      </CardContent>
    </Card>
  );
}

function DangerPanel({ role, onDeleted, onError }: {
  role: Role; onDeleted: () => void; onError: (e: string) => void;
}) {
  const [confirm, setConfirm] = useState(false);
  const del = async () => {
    try {
      await api.deleteRole(role.id);
      onDeleted();
    } catch (e) {
      onError((e as Error).message);
    }
  };
  return (
    <Card size="sm" className="mb-4 border-destructive/70">
      <CardContent>
      <h2 className="m-0 mb-3 text-base font-semibold text-destructive">Danger zone</h2>
      <p className="text-sm leading-relaxed text-muted-foreground">Users and group mappings holding this role lose its permissions immediately. This cannot be undone.</p>
      <Button variant="destructive" type="button" onClick={() => setConfirm(true)}>Delete role</Button>
      <ConfirmModal
        open={confirm}
        title={`Delete role "${role.name}"?`}
        message="Users and group mappings holding this role lose its permissions immediately."
        confirmLabel="Delete"
        danger
        onConfirm={() => { setConfirm(false); del(); }}
        onCancel={() => setConfirm(false)}
      />
      </CardContent>
    </Card>
  );
}
