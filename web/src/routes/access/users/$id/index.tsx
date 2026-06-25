import { ReactNode, useEffect, useState } from "react";
import { createFileRoute, Link, Navigate, useNavigate, useParams } from "@tanstack/react-router";
import { Eye, EyeOff, LockKeyhole, X } from "lucide-react";
import { api, Me, Role, Token, User } from "@/api";
import { useAuth } from "@/authContext";
import { ConfirmModal } from "@/components/overlays/confirm-modal";
import { Select } from "@/components/app-ui/select";
import { Alert } from "@/components/app-ui/alert";
import { PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrap,
} from "@/components/app-ui/table";
import { PermissionBadge, RoleBadge } from "@/components/app-ui/action-badge";
import { SourceBadge } from "@/components/app-ui/source-badge";
import { StateBadge } from "@/components/app-ui/status-badge";
import { UserBadge } from "@/components/app-ui/user-badge";
import { Toggle } from "@/components/inputs/toggle";
import { Button } from "@/components/ui/button";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/access/users/$id/")({
  component: UserModifyRoute,
});

function UserModifyRoute() {
  const { me } = useAuth();
  return me.admin || me.auditor ? <UserModify me={me} /> : <Navigate to="/workspace/repositories" replace />;
}

interface Scope {
  repo_pattern: string;
  actions: string[];
}

function parseScopes(json: string): Scope[] {
  try {
    const v = JSON.parse(json);
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}

// Per-user modify page: role mapping, password reset, enable/disable, and the
// danger zone (delete). The Users list is read-only; all edits happen here.
export function UserModify({ me }: { me: Me }) {
  const { id } = useParams({ strict: false }) as { id?: string };
  const navigate = useNavigate();
  const userId = Number(id);
  const [user, setUser] = useState<User | null>(null);
  const [roles, setRoles] = useState<Role[]>([]);
  const [tokens, setTokens] = useState<Token[]>([]);
  const [error, setError] = useState("");

  const load = () =>
    Promise.all([api.listUsers(), api.listRoles(), api.listUserTokens(userId)])
      .then(([users, rs, ts]) => {
        const u = users.find((x) => x.id === userId) ?? null;
        setUser(u);
        setRoles(rs);
        setTokens(ts);
        if (!u) setError("User not found.");
      })
      .catch((e) => setError(e.message));
  useEffect(() => { load(); /* eslint-disable-next-line */ }, [userId]);

  if (error && !user) return <Alert className="my-2.5">{error}</Alert>;
  if (!user) return <div className="text-sm text-muted-foreground">Loading…</div>;

  const self = user.username === me.username;

  const run = (p: Promise<unknown>) => {
    setError("");
    p.then(load).catch((e) => setError((e as Error).message));
  };

  return (
    <>
      <PageHeader
        title={
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <span className="min-w-0 truncate">{user.username}</span>
            <SourceBadge source={user.source} />
            {self && <UserBadge username="you">you</UserBadge>}
          </div>
        }
        actions={
          <Button render={<Link to="/access/users" />} nativeButton={false} variant="outline">
            Back to users
          </Button>
        }
      />
      {error && <Alert className="mb-4">{error}</Alert>}

      <AccountPanel user={user} />
      <RolesPanel user={user} roles={roles} run={run} canWrite={!!me.admin} />
      <TokensPanel user={user} tokens={tokens} canWrite={!!me.admin} run={run} />
      {me.admin && user.source === "local" && <PasswordPanel user={user} onError={setError} />}
      {me.admin && user.source === "local" && <LockoutPanel user={user} run={run} />}
      {me.admin && <StatusPanel user={user} self={self} run={run} />}
      {me.admin && <DangerPanel user={user} self={self} onDeleted={() => navigate({ to: "/access/users" })} onError={setError} />}
    </>
  );
}

// AccountPanel shows the identity fields read-only. Username and email are owned
// by the identity provider (OIDC) or set at creation (local), so they are not
// editable here — only displayed.
function AccountPanel({ user }: { user: User }) {
  return (
    <Card size="sm" className="mb-4">
      <CardContent>
        <h2 className="m-0 mb-4 text-base font-semibold">Account</h2>
        <FieldGroup className="grid gap-4 sm:grid-cols-2">
          <ReadOnlyField label="Username" value={user.username} />
          <ReadOnlyField label="Email" value={user.email || "—"} />
          <ReadOnlyField label="Created" value={user.created_at ? new Date(user.created_at).toLocaleString() : "—"} />
          <ReadOnlyField label="Last login" value={user.last_login_at ? new Date(user.last_login_at).toLocaleString() : "never"} />
        </FieldGroup>
      </CardContent>
    </Card>
  );
}

function ReadOnlyField({ label, value }: { label: string; value: string }) {
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      <Input value={value} readOnly className="bg-muted/20 text-muted-foreground" />
    </Field>
  );
}

function RolesPanel({ user, roles, run, canWrite }: { user: User; roles: Role[]; run: (p: Promise<unknown>) => void; canWrite: boolean }) {
  const [selected, setSelected] = useState("");
  const assignable = roles.filter((r) => !user.roles.some((ur) => ur.id === r.id));

  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-4 text-base font-semibold">Roles</h2>
      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        {user.roles.map((r) => (
          <RoleBadge key={r.id} className="gap-1">
            <span>{r.name}</span>
            {canWrite && (
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                className="-mr-1 size-4 rounded-full text-muted-foreground hover:bg-background/40 hover:text-foreground"
                title="Remove role"
                onClick={() => run(api.removeRole(user.id, r.id))}
              >
                <X className="size-3" aria-hidden="true" />
                <span className="sr-only">Remove role</span>
              </Button>
            )}
          </RoleBadge>
        ))}
        {user.roles.length === 0 && <span className="text-sm text-muted-foreground">No roles assigned.</span>}
      </div>
      {canWrite && assignable.length > 0 && (
        <div className="flex min-w-0 items-center gap-2 mt-4 max-sm:flex-wrap items-stretch max-sm:flex-col">
          <Select value={selected} onChange={setSelected} placeholder="add role…"
            options={assignable.map((r) => ({ value: String(r.id), label: r.name, description: r.description || undefined }))} />
          <Button variant="outline" type="button" disabled={!selected}
            onClick={() => { run(api.assignRole(user.id, Number(selected))); setSelected(""); }}>
            Add
          </Button>
        </div>
      )}
      </CardContent>
    </Card>
  );
}

// TokensPanel shows the user's personal access tokens. Admins can create
// (via the New token page) and revoke them; auditors see the list read-only.
// Token scopes only ever narrow the user's own role permissions, so issuing a
// token here cannot grant access the user does not already have.
function TokensPanel({ user, tokens, canWrite, run }: {
  user: User; tokens: Token[]; canWrite: boolean; run: (p: Promise<unknown>) => void;
}) {
  const [revokeId, setRevokeId] = useState<number | null>(null);

  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <div className="mb-4 flex items-start justify-between gap-3 max-sm:flex-col max-sm:items-stretch">
        <h2 className="m-0 text-base font-semibold">
          Access tokens <span className="text-xs font-normal text-muted-foreground">· scoped credentials for package clients</span>
        </h2>
        {canWrite && (
          <Button
            render={<Link to="/access/users/$id/tokens/new" params={{ id: String(user.id) }} />}
            nativeButton={false}
          >
            New token
          </Button>
        )}
      </div>
      <TableWrap>
        <Table>
        <TableHeader><TableRow><TableHead>Name</TableHead><TableHead>Description</TableHead><TableHead>Permissions</TableHead><TableHead>Created</TableHead><TableHead>Expires</TableHead><TableHead>Last used</TableHead>{canWrite && <TableHead></TableHead>}</TableRow></TableHeader>
        <TableBody>
          {tokens.map((t) => (
            <TableRow key={t.id}>
              <TableCell>{t.name}</TableCell>
              <TableCell className="text-muted-foreground">{t.description}</TableCell>
              <TableCell>
                {parseScopes(t.scopes_json).map((s, i) => (
                  <PermissionBadge key={i} className="mr-1">
                    {s.repo_pattern}: {s.actions.join(",")}
                  </PermissionBadge>
                ))}
              </TableCell>
              <TableCell className="text-muted-foreground">{t.created_at?.slice(0, 10)}</TableCell>
              <TableCell className="text-muted-foreground">{t.expires_at ? t.expires_at.slice(0, 10) : "never"}</TableCell>
              <TableCell className="text-muted-foreground">{t.last_used_at ? t.last_used_at.slice(0, 10) : "never"}</TableCell>
              {canWrite && <TableCell><Button variant="destructive" onClick={() => setRevokeId(t.id)}>Revoke</Button></TableCell>}
            </TableRow>
          ))}
          {tokens.length === 0 && <TableRow><TableCell colSpan={canWrite ? 7 : 6} className="text-muted-foreground">No tokens yet.</TableCell></TableRow>}
        </TableBody>
        </Table>
      </TableWrap>
      {!canWrite && <p className="mb-0 mt-3 text-sm text-muted-foreground">Read-only: only administrators can create or revoke tokens.</p>}
      <ConfirmModal
        open={revokeId !== null}
        title="Revoke this token?"
        message="Clients using this token will immediately lose access. This cannot be undone."
        confirmLabel="Revoke"
        danger
        onConfirm={() => { if (revokeId !== null) run(api.deleteUserToken(user.id, revokeId)); setRevokeId(null); }}
        onCancel={() => setRevokeId(null)}
      />
      </CardContent>
    </Card>
  );
}

function PasswordPanel({ user, onError }: { user: User; onError: (e: string) => void }) {
  const [password, setPassword] = useState("");
  const [show, setShow] = useState(false);
  const [saved, setSaved] = useState(false);

  const reset = async () => {
    onError("");
    setSaved(false);
    try {
      await api.updateUser(user.id, { password });
      setPassword("");
      setSaved(true);
    } catch (e) {
      onError((e as Error).message);
    }
  };

  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-4 text-base font-semibold">Password</h2>
      <Field>
        <FieldLabel>New password</FieldLabel>
        <div className="relative">
        <Input className="pr-16" type={show ? "text" : "password"} value={password}
          onChange={(e) => { setPassword(e.target.value); setSaved(false); }} />
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="absolute right-0 top-0 h-full rounded-l-none text-muted-foreground"
          onClick={() => setShow((s) => !s)}
          aria-label={show ? "Hide password" : "Show password"}
        >
          {show ? <EyeOff className="size-4" aria-hidden="true" /> : <Eye className="size-4" aria-hidden="true" />}
        </Button>
        </div>
      </Field>
      <div className="flex min-w-0 items-center gap-2 mt-4 max-sm:flex-wrap max-sm:flex-col max-sm:items-stretch">
        <Button type="button" disabled={!password} onClick={reset}>Reset password</Button>
        {saved && <span className="text-sm text-muted-foreground">Password updated.</span>}
      </div>
      </CardContent>
    </Card>
  );
}

// LockoutPanel toggles failed-password lockout for a local account and unlocks
// it after a lockout. The default admin is protected: the toggle is disabled so
// it can never be locked out of the only guaranteed admin account.
function LockoutPanel({ user, run }: { user: User; run: (p: Promise<unknown>) => void }) {
  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-3 text-base font-semibold">Account lockout</h2>
      <p className="text-sm leading-relaxed text-muted-foreground">
        When enabled, the account is locked after 5 consecutive failed password attempts and must be
        unlocked by an administrator. {user.protected && "The default admin account cannot be locked out."}
      </p>
      <Toggle
        checked={user.lockout_enabled}
        disabled={user.protected}
        label={user.lockout_enabled ? "Lockout enabled" : "Lockout disabled"}
        onChange={(v) => run(api.updateUser(user.id, { lockout_enabled: v }))}
      />
      {user.locked && (
        <div className="flex min-w-0 items-center gap-2 mt-4 max-sm:flex-wrap max-sm:flex-col max-sm:items-stretch">
          <StateBadge state="locked">Locked</StateBadge>
          <Button type="button"
            onClick={() => run(api.updateUser(user.id, { unlock: true }))}>
            Unlock account
          </Button>
        </div>
      )}
      </CardContent>
    </Card>
  );
}

// LockNote renders an accent-bordered callout with a lock icon, matching the
// managed-role notice. Used to explain why an edit control is locked.
function LockNote({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="mt-4 rounded-lg border border-primary/70 bg-primary/5 p-4">
      <h2 className="mb-2 flex items-center gap-2 text-[15px] font-semibold">
        <LockKeyhole className="size-4 text-primary" aria-hidden="true" />
        {title}
      </h2>
      <p className="m-0 text-sm leading-relaxed text-muted-foreground">{children}</p>
    </div>
  );
}

function StatusPanel({ user, self, run }: { user: User; self: boolean; run: (p: Promise<unknown>) => void }) {
  const lockedFromEditing = self || user.protected;
  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-3 text-base font-semibold">Status</h2>
      <p className="text-sm leading-relaxed text-muted-foreground">
        An <strong>active</strong> account can sign in to Forklift and use its credentials to pull and publish
        artifacts, while <strong>disabling</strong> immediately revokes that access by stopping all sessions and
        tokens. The account, its tokens and its role assignments are preserved, so disabling is a reversible
        alternative to deletion that you can undo at any time by re-enabling.
      </p>
      <Toggle
        checked={!user.disabled}
        disabled={lockedFromEditing}
        label={user.disabled ? "Account disabled" : "Account active"}
        onChange={(v) => run(api.updateUser(user.id, { disabled: !v }))}
      />
      {user.protected ? (
        <LockNote title="Status locked">
          This is the default administrator account, created when Forklift first started. Its status is locked
          active so the system always keeps at least one administrator who can sign in and recover access. It
          cannot be disabled by anyone, including other administrators.
        </LockNote>
      ) : self && (
        <LockNote title="Status locked">
          You are signed in as this account, so you cannot disable it yourself. This guards against accidentally
          locking yourself out of Forklift. If this account needs to be disabled, ask another administrator to
          do it.
        </LockNote>
      )}
      {user.locked && (
        <p className="mb-0 mt-3">
          <StateBadge state="locked">Locked</StateBadge>
          <span className="ml-2 text-sm text-muted-foreground">
            Locked after too many failed password attempts — unlock it in Account lockout.
          </span>
        </p>
      )}
      </CardContent>
    </Card>
  );
}

function DangerPanel({ user, self, onDeleted, onError }: {
  user: User; self: boolean; onDeleted: () => void; onError: (e: string) => void;
}) {
  const [confirm, setConfirm] = useState(false);
  const del = async () => {
    try {
      await api.deleteUser(user.id);
      onDeleted();
    } catch (e) {
      onError((e as Error).message);
    }
  };
  return (
    <Card size="sm" className="mb-4 border-destructive/70">
      <CardContent>
      <h2 className="m-0 mb-3 text-base font-semibold text-destructive">Danger zone</h2>
      <p className="text-sm leading-relaxed text-muted-foreground">
        Deleting a user revokes all of their tokens and role assignments. This cannot be undone.
        {self && " You cannot delete your own account."}
      </p>
      <Button variant="destructive" type="button" disabled={self} onClick={() => setConfirm(true)}>Delete user</Button>
      <ConfirmModal
        open={confirm}
        title={`Delete user "${user.username}"?`}
        message="This revokes all of the user's tokens and role assignments. This cannot be undone."
        confirmLabel="Delete"
        danger
        onConfirm={() => { setConfirm(false); del(); }}
        onCancel={() => setConfirm(false)}
      />
      </CardContent>
    </Card>
  );
}
