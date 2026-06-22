import { useEffect, useState } from "react";
import { createFileRoute, Link, Navigate, useNavigate, useParams } from "@tanstack/react-router";
import { api, Me, Role, User } from "../../api";
import { useAuth } from "../../authContext";
import { ConfirmModal } from "../../components/confirm-modal";
import { Combobox } from "../../components/combobox";
import { CountBadge, StateBadge } from "@/components/app-ui/status-badge";
import { PermissionBadge, RoleBadge } from "@/components/app-ui/action-badge";
import { SourceBadge } from "@/components/app-ui/source-badge";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/roles/$id")({
  component: RoleModifyRoute,
});

function RoleModifyRoute() {
  const { me } = useAuth();
  return me.admin || me.auditor ? <RoleModify me={me} /> : <Navigate to="/repositories" replace />;
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

  if (error && !role) return <div className="my-2.5 rounded-[var(--radius)] border border-[color-mix(in_oklch,var(--danger)_48%,var(--border))] bg-[color-mix(in_oklch,var(--panel-2)_88%,var(--danger)_12%)] px-[11px] py-[9px] text-foreground">{error}</div>;
  if (!role) return <div>Loading…</div>;

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
      <div className="mb-[18px] flex items-center justify-between gap-3 max-[760px]:flex-col max-[760px]:items-start [&_h1]:m-0">
        <h1>{role.name}</h1>
        <Button render={<Link to="/roles" />} nativeButton={false} variant="outline">
          Back to roles
        </Button>
      </div>
      {role.description && <p className="-mt-2 mb-[22px] max-w-[820px] text-[13px] leading-[1.55] text-muted-foreground">{role.description}</p>}
      {role.managed && (
        <div className="mb-[18px] rounded-[10px] border border-border bg-[linear-gradient(180deg,color-mix(in_oklch,var(--panel)_96%,#fff_4%),var(--panel))] p-[18px] shadow-[inset_0_1px_0_rgba(255,255,255,0.025)]" style={{ borderColor: "var(--accent)" }}>
          <h2 style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor"
              strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"
              style={{ color: "var(--accent)" }}>
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
            Managed role
          </h2>
          <p className="muted" style={{ margin: 0 }}>
            This role was configured by a Forklift administrator in the declarative RBAC policy so it cannot be edited here. To change its permissions or delete it ask an administrator to update the policy file and restart forklift.
          </p>
        </div>
      )}
      {error && <div className="my-2.5 rounded-[var(--radius)] border border-[color-mix(in_oklch,var(--danger)_48%,var(--border))] bg-[color-mix(in_oklch,var(--panel-2)_88%,var(--danger)_12%)] px-[11px] py-[9px] text-foreground">{error}</div>}

      <PermissionsPanel role={role} run={run} canWrite={editable} />
      <AssignedUsersPanel members={members} />
      {editable && <DangerPanel role={role} onDeleted={() => navigate({ to: "/roles" })} onError={setError} />}
    </>
  );
}

// AssignedUsersPanel lists the users that currently hold this role. Assignment
// itself is managed on each user's detail page, so this is read-only with links.
function AssignedUsersPanel({ members }: { members: User[] }) {
  return (
      <div className="mb-[18px] rounded-[10px] border border-border bg-[linear-gradient(180deg,color-mix(in_oklch,var(--panel)_96%,#fff_4%),var(--panel))] p-[18px] shadow-[inset_0_1px_0_rgba(255,255,255,0.025)]">
      <h2>
        Assigned users <CountBadge className="ml-1.5">{members.length}</CountBadge>
      </h2>
      {members.length === 0
        ? <p className="muted">No users have this role. Assign it from a user's detail page.</p>
        : (
          // Same column structure and order as the Users page; the username
          // links to that user's detail page.
          <table>
            <thead>
              <tr><th>Username</th><th>Source</th><th>Email</th><th>Roles</th><th>Status</th><th>Last login</th></tr>
            </thead>
            <tbody>
              {members.map((u) => (
                <tr key={u.id}>
                  <td style={{ whiteSpace: "nowrap" }}><Link to="/users/$id" params={{ id: String(u.id) }}>{u.username}</Link></td>
                  <td><SourceBadge source={u.source} /></td>
                  <td className="muted">{u.email || "-"}</td>
                  <td>
                    <div className="flex items-center gap-2.5 max-[760px]:flex-col max-[760px]:items-stretch" style={{ flexWrap: "wrap", gap: 6 }}>
                      {u.roles.map((r) => (
                        <RoleBadge
                          key={r.id}
                          render={<Link to="/roles/$id" params={{ id: String(r.id) }} />}
                        >
                          {r.name}
                        </RoleBadge>
                      ))}
                      {u.roles.length === 0 && <span className="muted">none</span>}
                    </div>
                  </td>
                  <td>
                    {u.disabled
                      ? <StateBadge state="disabled">disabled</StateBadge>
                      : <StateBadge state="active">active</StateBadge>}
                  </td>
                  <td className="muted" style={{ whiteSpace: "nowrap" }} title={u.last_login_at ?? undefined}>
                    {u.last_login_at ? new Date(u.last_login_at).toLocaleString() : "never"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
    </div>
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
    <div className="mb-[18px] rounded-[10px] border border-border bg-[linear-gradient(180deg,color-mix(in_oklch,var(--panel)_96%,#fff_4%),var(--panel))] p-[18px] shadow-[inset_0_1px_0_rgba(255,255,255,0.025)]">
      <h2>Permissions</h2>
      <div className="flex items-center gap-2.5 max-[760px]:flex-col max-[760px]:items-stretch" style={{ flexWrap: "wrap", gap: 6 }}>
        {role.permissions.map((p) => (
          <PermissionBadge key={p.id}>
            {p.repo_pattern}: {p.actions.join(",")}
            {canWrite && (
              <a style={{ marginLeft: 6, cursor: "pointer" }} title="Remove permission"
                onClick={() => run(api.deletePermission(role.id, p.id))}>×</a>
            )}
          </PermissionBadge>
        ))}
        {role.permissions.length === 0 && <span className="muted">No permissions granted.</span>}
      </div>
      {canWrite && (
        <div className="flex items-center gap-2.5 max-[760px]:flex-col max-[760px]:items-stretch" style={{ marginTop: 12, flexWrap: "wrap", gap: 8 }}>
          <Combobox style={{ width: 200 }} value={pattern} onChange={setPattern}
            options={repoOptions} hints={repoTypes} placeholder="repo pattern (* or maven-*)" />
          {ACTIONS.map((a) => (
            <label key={a} className="flex items-center gap-2 [&_input]:w-auto" style={{ margin: 0, fontSize: 12 }}>
              <input type="checkbox" checked={actions.includes(a)} onChange={() => toggle(a)} />
              <span>{a}</span>
            </label>
          ))}
          <Button variant="outline" type="button"
            disabled={!pattern.trim() || actions.length === 0} onClick={add}>Add</Button>
        </div>
      )}
    </div>
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
    <div className="mb-[18px] rounded-[10px] border border-destructive bg-[linear-gradient(180deg,color-mix(in_oklch,var(--panel)_96%,#fff_4%),var(--panel))] p-[18px] shadow-[inset_0_1px_0_rgba(255,255,255,0.025)] [&_h2]:text-destructive" style={{ marginTop: 18 }}>
      <h2>Danger zone</h2>
      <p className="muted">Users and group mappings holding this role lose its permissions immediately. This cannot be undone.</p>
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
    </div>
  );
}
